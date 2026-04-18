//go:build e2e

package e2e

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/gmtantsevov/shuggabuddy/internal/delivery/telegram"
	"github.com/gmtantsevov/shuggabuddy/internal/i18n"
	postgresrepo "github.com/gmtantsevov/shuggabuddy/internal/repository/postgres"
	"github.com/gmtantsevov/shuggabuddy/internal/usecase/activity"
	"github.com/gmtantsevov/shuggabuddy/internal/usecase/bolus"
	"github.com/gmtantsevov/shuggabuddy/internal/usecase/cgm"
	"github.com/gmtantsevov/shuggabuddy/internal/usecase/diary"
	"github.com/gmtantsevov/shuggabuddy/internal/usecase/doseadvisor"
	"github.com/gmtantsevov/shuggabuddy/internal/usecase/food"
	"github.com/gmtantsevov/shuggabuddy/internal/usecase/glucose"
	"github.com/gmtantsevov/shuggabuddy/internal/usecase/insulin"
	"github.com/gmtantsevov/shuggabuddy/internal/usecase/note"
	"github.com/gmtantsevov/shuggabuddy/internal/usecase/user"
	"github.com/gmtantsevov/shuggabuddy/pkg/nightscout"
)

type stubEncryptor struct{}

func (s *stubEncryptor) Encrypt(plaintext string) (string, error) { return "enc:" + plaintext, nil }
func (s *stubEncryptor) Decrypt(ciphertext string) (string, error) {
	if len(ciphertext) > 4 && ciphertext[:4] == "enc:" {
		return ciphertext[4:], nil
	}
	return ciphertext, nil
}

func newTestEnvWithCGM(t *testing.T, cgmUC *cgm.UseCase) *testEnv {
	t.Helper()
	truncateTables(t)

	bot := newSpyBot()

	userRepo := postgresrepo.NewUserRepo(testPool)
	extAccRepo := postgresrepo.NewExternalAccountRepo(testPool)
	glucoseRepo := postgresrepo.NewGlucoseRepo(testPool)
	foodRepo := postgresrepo.NewFoodRepo(testPool)
	insulinRepo := postgresrepo.NewInsulinRepo(testPool)
	activityRepo := postgresrepo.NewActivityRepo(testPool)
	reminderRepo := postgresrepo.NewReminderRepo(testPool)
	noteRepo := postgresrepo.NewNoteRepository(testPool)

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
		cgmUC, loc, zap.NewNop(),
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

	env.sendStart()
	env.userID = env.getUserID()
	env.bot.reset()

	return env
}

func TestCGMConnectAndSync(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Millisecond)
	entries := []nightscout.Entry{
		{SGV: 120, Direction: "Flat", DateMs: now.Add(-5 * time.Minute).UnixMilli()},
		{SGV: 180, Direction: "SingleUp", DateMs: now.UnixMilli()},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/status.json" {
			w.Write([]byte(`{"status":"ok"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(entries)
	}))
	defer srv.Close()

	glucoseRepo := postgresrepo.NewGlucoseRepo(testPool)
	cgmRepo := postgresrepo.NewCGMConnectionRepo(testPool)
	cgmUC := cgm.New(cgmRepo, glucoseRepo, &stubEncryptor{})

	env := newTestEnvWithCGM(t, cgmUC)

	// 1. Open CGM section from profile — should show intro.
	env.sendCallback("profile:cgm")
	msg := env.bot.lastMessageText()
	assert.Contains(t, msg, "Nightscout")
	assert.Contains(t, msg, "Подключить")

	// 2. Start connect flow.
	env.bot.reset()
	env.sendCallback("cgm:connect")
	msg = env.bot.lastMessageText()
	assert.Contains(t, msg, "URL")

	// 3. Enter URL.
	env.bot.reset()
	env.sendText(srv.URL)
	msg = env.bot.lastMessageText()
	assert.Contains(t, msg, "API Secret")

	// 4. Enter token — should connect successfully.
	env.bot.reset()
	env.sendText("testsecret")
	msg = env.bot.lastMessageText()
	assert.Contains(t, msg, "подключён")

	// Verify connection in DB.
	cnt := env.queryInt(
		`SELECT count(*) FROM cgm_connections WHERE user_id = $1 AND active = TRUE`,
		env.userID,
	)
	assert.Equal(t, int64(1), cnt)

	// 5. Trigger sync manually via usecase.
	conn, err := cgmRepo.GetByUserID(context.Background(), env.userID)
	require.NoError(t, err)
	require.NotNil(t, conn)

	err = cgmUC.SyncUser(context.Background(), conn)
	require.NoError(t, err)

	// Verify glucose readings in DB with source=nightscout and non-null trend.
	glucoseCnt := env.queryInt(
		`SELECT count(*) FROM glucose_readings WHERE user_id = $1 AND source = 'nightscout'`,
		env.userID,
	)
	assert.Equal(t, int64(2), glucoseCnt)

	trendCnt := env.queryInt(
		`SELECT count(*) FROM glucose_readings WHERE user_id = $1 AND trend IS NOT NULL`,
		env.userID,
	)
	assert.Equal(t, int64(2), trendCnt)

	// 6. Disconnect.
	env.bot.reset()
	env.sendCallback("profile:cgm")
	msg = env.bot.lastMessageText()
	assert.Contains(t, msg, "подключён")

	env.bot.reset()
	env.sendCallback("cgm:disconnect")
	msg = env.bot.lastMessageText()
	assert.Contains(t, msg, "Отключить")

	env.bot.reset()
	env.sendCallback("cgm:disconnect:yes")
	msg = env.bot.lastMessageText()
	assert.Contains(t, msg, "отключён")

	// Verify connection removed.
	cnt = env.queryInt(
		`SELECT count(*) FROM cgm_connections WHERE user_id = $1`,
		env.userID,
	)
	assert.Equal(t, int64(0), cnt)

	// Glucose readings should still exist (data persists after disconnect).
	glucoseCnt = env.queryInt(
		`SELECT count(*) FROM glucose_readings WHERE user_id = $1 AND source = 'nightscout'`,
		env.userID,
	)
	assert.Equal(t, int64(2), glucoseCnt)
}

func TestCGMConnectAuthFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	glucoseRepo := postgresrepo.NewGlucoseRepo(testPool)
	cgmRepo := postgresrepo.NewCGMConnectionRepo(testPool)
	cgmUC := cgm.New(cgmRepo, glucoseRepo, &stubEncryptor{})

	env := newTestEnvWithCGM(t, cgmUC)

	env.sendCallback("profile:cgm")
	env.sendCallback("cgm:connect")
	env.sendText(srv.URL)
	env.bot.reset()
	env.sendText("bad_secret")

	msg := env.bot.lastMessageText()
	assert.Contains(t, msg, "авторизации")

	// No connection should be saved.
	cnt := env.queryInt(
		`SELECT count(*) FROM cgm_connections WHERE user_id = $1`,
		env.userID,
	)
	assert.Equal(t, int64(0), cnt)
}

func TestCGMShowsIntroWhenNotConnected(t *testing.T) {
	glucoseRepo := postgresrepo.NewGlucoseRepo(testPool)
	cgmRepo := postgresrepo.NewCGMConnectionRepo(testPool)
	cgmUC := cgm.New(cgmRepo, glucoseRepo, &stubEncryptor{})

	env := newTestEnvWithCGM(t, cgmUC)

	env.sendCallback("profile:cgm")
	msg := env.bot.lastMessageText()

	assert.Contains(t, msg, "Nightscout")
	assert.Contains(t, msg, "xDrip")
	assert.Contains(t, msg, "FreeStyle Libre")
	assert.Contains(t, msg, "API Secret")
}
