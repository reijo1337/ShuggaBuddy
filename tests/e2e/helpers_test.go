//go:build e2e

package e2e

import (
	"context"
	"strconv"
	"sync"
	"testing"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/gmtantsevov/shuggabuddy/internal/delivery/telegram"
	"github.com/gmtantsevov/shuggabuddy/internal/i18n"
	postgresrepo "github.com/gmtantsevov/shuggabuddy/internal/repository/postgres"
	"github.com/gmtantsevov/shuggabuddy/internal/usecase/activity"
	"github.com/gmtantsevov/shuggabuddy/internal/usecase/bolus"
	"github.com/gmtantsevov/shuggabuddy/internal/usecase/diary"
	"github.com/gmtantsevov/shuggabuddy/internal/usecase/doseadvisor"
	"github.com/gmtantsevov/shuggabuddy/internal/usecase/food"
	"github.com/gmtantsevov/shuggabuddy/internal/usecase/glucose"
	"github.com/gmtantsevov/shuggabuddy/internal/usecase/insulin"
	"github.com/gmtantsevov/shuggabuddy/internal/usecase/note"
	"github.com/gmtantsevov/shuggabuddy/internal/usecase/user"
)

// ---------------------------------------------------------------------------
// spyBot — fake BotAPI that captures sent messages and provides a controlled
// updates channel for Handler.Run().
// ---------------------------------------------------------------------------

type spyBot struct {
	mu       sync.Mutex
	sent     []tgbotapi.Chattable
	requests []tgbotapi.Chattable
	updates  chan tgbotapi.Update
}

func newSpyBot() *spyBot {
	return &spyBot{updates: make(chan tgbotapi.Update)} // unbuffered
}

func (s *spyBot) Send(c tgbotapi.Chattable) (tgbotapi.Message, error) {
	s.mu.Lock()
	s.sent = append(s.sent, c)
	s.mu.Unlock()
	return tgbotapi.Message{}, nil
}

func (s *spyBot) Request(c tgbotapi.Chattable) (*tgbotapi.APIResponse, error) {
	s.mu.Lock()
	s.requests = append(s.requests, c)
	s.mu.Unlock()
	return &tgbotapi.APIResponse{Ok: true}, nil
}

func (s *spyBot) GetUpdatesChan(_ tgbotapi.UpdateConfig) tgbotapi.UpdatesChannel {
	return s.updates
}

func (s *spyBot) StopReceivingUpdates() {}

// lastMessageText returns the text of the last sent message.
func (s *spyBot) lastMessageText() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := len(s.sent) - 1; i >= 0; i-- {
		switch v := s.sent[i].(type) {
		case tgbotapi.MessageConfig:
			return v.Text
		case tgbotapi.EditMessageTextConfig:
			return v.Text
		}
	}
	return ""
}

// sentCount returns the number of sent messages.
func (s *spyBot) sentCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.sent)
}

// reset clears all recorded messages.
func (s *spyBot) reset() {
	s.mu.Lock()
	s.sent = nil
	s.requests = nil
	s.mu.Unlock()
}

// ---------------------------------------------------------------------------
// testEnv — full-stack test environment.
// ---------------------------------------------------------------------------

type testEnv struct {
	t          *testing.T
	bot        *spyBot
	pool       *pgxpool.Pool
	ctx        context.Context
	cancel     context.CancelFunc
	telegramID int64
	chatID     int64
	userID     int64
}

func newTestEnv(t *testing.T, withUser bool) *testEnv {
	t.Helper()

	truncateTables(t)

	bot := newSpyBot()

	// Repositories
	userRepo := postgresrepo.NewUserRepo(testPool)
	extAccRepo := postgresrepo.NewExternalAccountRepo(testPool)
	glucoseRepo := postgresrepo.NewGlucoseRepo(testPool)
	foodRepo := postgresrepo.NewFoodRepo(testPool)
	insulinRepo := postgresrepo.NewInsulinRepo(testPool)
	activityRepo := postgresrepo.NewActivityRepo(testPool)
	reminderRepo := postgresrepo.NewReminderRepo(testPool)
	noteRepo := postgresrepo.NewNoteRepository(testPool)

	// Usecases
	userUC := user.New(userRepo, extAccRepo)
	glucoseUC := glucose.New(glucoseRepo)
	foodUC := food.New(foodRepo)
	insulinUC := insulin.New(insulinRepo)
	activityUC := activity.New(activityRepo, glucoseRepo, reminderRepo)
	noteUC := note.New(noteRepo)
	diaryUC := diary.New(glucoseRepo, foodRepo, insulinRepo, activityRepo, noteRepo)
	bolusUC := bolus.New(userRepo, insulinRepo, glucoseRepo, foodRepo)
	advisorUC := doseadvisor.New(userRepo, insulinRepo, glucoseRepo, foodRepo)

	loc, err := i18n.NewLocalizer("../../locales", "ru")
	require.NoError(t, err)

	handler := telegram.NewHandler(
		bot, userUC, glucoseUC, foodUC, insulinUC,
		activityUC, noteUC, diaryUC, bolusUC, advisorUC,
		loc, zap.NewNop(),
	)

	ctx, cancel := context.WithCancel(context.Background())
	go handler.Run(ctx)

	env := &testEnv{
		t:          t,
		bot:        bot,
		pool:       testPool,
		ctx:        ctx,
		cancel:     cancel,
		telegramID: 100_000 + time.Now().UnixNano()%1_000_000,
		chatID:     200_000 + time.Now().UnixNano()%1_000_000,
	}

	t.Cleanup(func() { cancel() })

	if withUser {
		env.sendStart()
		env.userID = env.getUserID()
		env.bot.reset()
	}

	return env
}

// ---------------------------------------------------------------------------
// Update helpers
// ---------------------------------------------------------------------------

func (env *testEnv) processUpdate(update tgbotapi.Update) {
	env.t.Helper()
	select {
	case env.bot.updates <- update:
	case <-time.After(5 * time.Second):
		env.t.Fatal("timeout sending update to handler")
	}
	select {
	case env.bot.updates <- tgbotapi.Update{}:
	case <-time.After(5 * time.Second):
		env.t.Fatal("timeout waiting for handler to finish processing")
	}
}

func (env *testEnv) sendStart() {
	env.t.Helper()
	env.processUpdate(tgbotapi.Update{
		Message: &tgbotapi.Message{
			From: &tgbotapi.User{ID: env.telegramID, FirstName: "E2ETest"},
			Chat: &tgbotapi.Chat{ID: env.chatID},
			Text: "/start",
			Entities: []tgbotapi.MessageEntity{
				{Type: "bot_command", Offset: 0, Length: 6},
			},
		},
	})
}

func (env *testEnv) sendCallback(data string) {
	env.t.Helper()
	env.processUpdate(tgbotapi.Update{
		CallbackQuery: &tgbotapi.CallbackQuery{
			ID:   "cb_test",
			From: &tgbotapi.User{ID: env.telegramID, FirstName: "E2ETest"},
			Message: &tgbotapi.Message{
				Chat: &tgbotapi.Chat{ID: env.chatID},
			},
			Data: data,
		},
	})
}

func (env *testEnv) sendText(text string) {
	env.t.Helper()
	env.processUpdate(tgbotapi.Update{
		Message: &tgbotapi.Message{
			From: &tgbotapi.User{ID: env.telegramID, FirstName: "E2ETest"},
			Chat: &tgbotapi.Chat{ID: env.chatID},
			Text: text,
		},
	})
}

func (env *testEnv) getUserID() int64 {
	env.t.Helper()
	var id int64
	err := env.pool.QueryRow(env.ctx,
		`SELECT u.id FROM users u
		 JOIN external_accounts ea ON ea.user_id = u.id
		 WHERE ea.external_id = $1 AND ea.provider = 'telegram'`,
		strconv.FormatInt(env.telegramID, 10),
	).Scan(&id)
	require.NoError(env.t, err, "user must exist after /start")
	return id
}

func (env *testEnv) execSQL(query string, args ...any) {
	env.t.Helper()
	_, err := env.pool.Exec(env.ctx, query, args...)
	require.NoError(env.t, err)
}

func (env *testEnv) queryInt(query string, args ...any) int64 {
	env.t.Helper()
	var val int64
	err := env.pool.QueryRow(env.ctx, query, args...).Scan(&val)
	require.NoError(env.t, err)
	return val
}

func (env *testEnv) queryFloat(query string, args ...any) float64 {
	env.t.Helper()
	var val float64
	err := env.pool.QueryRow(env.ctx, query, args...).Scan(&val)
	require.NoError(env.t, err)
	return val
}

// ---------------------------------------------------------------------------
// DB cleanup
// ---------------------------------------------------------------------------

func truncateTables(t *testing.T) {
	t.Helper()
	_, err := testPool.Exec(context.Background(),
		`TRUNCATE reminders, notes, activity_entries, insulin_doses,
		         food_entries, glucose_readings, external_accounts, users
		 CASCADE`)
	require.NoError(t, err)
}
