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

	// 1. Open CGM section — should show provider selection.
	env.sendCallback("profile:cgm")
	msg := env.bot.lastMessageText()
	assert.Contains(t, msg, "CGM")

	// 2. Select Nightscout provider — should show intro.
	env.bot.reset()
	env.sendCallback("cgm:provider:nightscout")
	msg = env.bot.lastMessageText()
	assert.Contains(t, msg, "Nightscout")
	assert.Contains(t, msg, "Подключить")

	// 3. Start connect flow.
	env.bot.reset()
	env.sendCallback("cgm:connect")
	msg = env.bot.lastMessageText()
	assert.Contains(t, msg, "URL")

	// 4. Enter URL.
	env.bot.reset()
	env.sendText(srv.URL)
	msg = env.bot.lastMessageText()
	assert.Contains(t, msg, "API Secret")

	// 5. Enter token — should connect successfully.
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

	// 6. Trigger sync manually via usecase.
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

	// 7. Disconnect.
	env.bot.reset()
	env.sendCallback("profile:cgm")
	msg = env.bot.lastMessageText()
	assert.Contains(t, msg, "Nightscout")

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
	env.sendCallback("cgm:provider:nightscout")
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

func TestCGMShowsProviderSelectionWhenNotConnected(t *testing.T) {
	glucoseRepo := postgresrepo.NewGlucoseRepo(testPool)
	cgmRepo := postgresrepo.NewCGMConnectionRepo(testPool)
	cgmUC := cgm.New(cgmRepo, glucoseRepo, &stubEncryptor{})

	env := newTestEnvWithCGM(t, cgmUC)

	// Opening CGM section should show provider selection.
	env.sendCallback("profile:cgm")
	msg := env.bot.lastMessageText()
	assert.Contains(t, msg, "CGM")
	assert.Contains(t, msg, "сенсор")

	// Selecting Nightscout shows its intro.
	env.bot.reset()
	env.sendCallback("cgm:provider:nightscout")
	msg = env.bot.lastMessageText()
	assert.Contains(t, msg, "Nightscout")
	assert.Contains(t, msg, "xDrip")

	// Going back and selecting LibreLinkUp shows its intro.
	env.bot.reset()
	env.sendCallback("cgm:back")
	env.bot.reset()
	env.sendCallback("cgm:provider:librelinkup")
	msg = env.bot.lastMessageText()
	assert.Contains(t, msg, "LibreLinkUp")
	assert.Contains(t, msg, "LibreView")
}

func TestCGMLibreLinkUpConnectAndSync(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)

	// Mock LibreLinkUp API: login, connections, graph.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/llu/auth/login":
			json.NewEncoder(w).Encode(map[string]any{
				"status": 0,
				"data": map[string]any{
					"redirect": false,
					"authTicket": map[string]any{
						"token":   "test-jwt-token",
						"expires": time.Now().Add(time.Hour).Unix(),
					},
					"user": map[string]any{"id": "user-1"},
				},
			})
		case "/llu/connections":
			json.NewEncoder(w).Encode(map[string]any{
				"status": 0,
				"data": []map[string]any{
					{"patientId": "patient-1"},
				},
			})
		case "/llu/connections/patient-1/graph":
			json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"graphData": []map[string]any{
						{
							"FactoryTimestamp": now.Add(-10 * time.Minute).Format("1/2/2006 3:04:05 PM"),
							"ValueInMgPerDl":   110,
							"TrendArrow":       3,
						},
						{
							"FactoryTimestamp": now.Format("1/2/2006 3:04:05 PM"),
							"ValueInMgPerDl":   140,
							"TrendArrow":       4,
						},
					},
				},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	glucoseRepo := postgresrepo.NewGlucoseRepo(testPool)
	cgmRepo := postgresrepo.NewCGMConnectionRepo(testPool)
	cgmUC := cgm.New(cgmRepo, glucoseRepo, &stubEncryptor{}, cgm.WithLibreLinkUpBaseURL(srv.URL))

	env := newTestEnvWithCGM(t, cgmUC)

	// 1. Open CGM section — provider selection.
	env.sendCallback("profile:cgm")
	msg := env.bot.lastMessageText()
	assert.Contains(t, msg, "CGM")

	// 2. Select LibreLinkUp.
	env.bot.reset()
	env.sendCallback("cgm:provider:librelinkup")
	msg = env.bot.lastMessageText()
	assert.Contains(t, msg, "LibreLinkUp")

	// 3. Start connect flow.
	env.bot.reset()
	env.sendCallback("cgm:llu:connect")
	msg = env.bot.lastMessageText()
	assert.Contains(t, msg, "email")

	// 4. Enter email.
	env.bot.reset()
	env.sendText("test@example.com")
	msg = env.bot.lastMessageText()
	assert.Contains(t, msg, "пароль")

	// 5. Enter password — AddConnection hits the mock server and succeeds.
	env.bot.reset()
	env.sendText("testpassword")
	msg = env.bot.lastMessageText()
	assert.Contains(t, msg, "подключён")

	// Verify the connection was saved by the flow.
	conn, err := cgmRepo.GetByUserID(context.Background(), env.userID)
	require.NoError(t, err)
	require.NotNil(t, conn)
	assert.Equal(t, "librelinkup", string(conn.Provider))

	cnt := env.queryInt(
		`SELECT count(*) FROM cgm_connections WHERE user_id = $1 AND provider = 'librelinkup' AND active = TRUE`,
		env.userID,
	)
	assert.Equal(t, int64(1), cnt)

	// 6. Trigger sync via usecase against the mock server.
	err = cgmUC.SyncUser(context.Background(), conn)
	require.NoError(t, err)

	glucoseCnt := env.queryInt(
		`SELECT count(*) FROM glucose_readings WHERE user_id = $1 AND source = 'librelinkup'`,
		env.userID,
	)
	assert.Equal(t, int64(2), glucoseCnt)

	trendCnt := env.queryInt(
		`SELECT count(*) FROM glucose_readings WHERE user_id = $1 AND trend IS NOT NULL`,
		env.userID,
	)
	assert.Equal(t, int64(2), trendCnt)

	// 7. Disconnect LibreLinkUp.
	env.bot.reset()
	env.sendCallback("profile:cgm")
	env.bot.reset()
	env.sendCallback("cgm:disconnect")
	msg = env.bot.lastMessageText()
	assert.Contains(t, msg, "Отключить")

	env.bot.reset()
	env.sendCallback("cgm:disconnect:yes")
	msg = env.bot.lastMessageText()
	assert.Contains(t, msg, "отключён")

	cnt = env.queryInt(
		`SELECT count(*) FROM cgm_connections WHERE user_id = $1`,
		env.userID,
	)
	assert.Equal(t, int64(0), cnt)
}

func TestCGMLibreLinkUpAuthFailure(t *testing.T) {
	// Mock LibreLinkUp API rejecting the credentials (status 2 = unauthorized).
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/llu/auth/login" {
			json.NewEncoder(w).Encode(map[string]any{"status": 2})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	glucoseRepo := postgresrepo.NewGlucoseRepo(testPool)
	cgmRepo := postgresrepo.NewCGMConnectionRepo(testPool)
	cgmUC := cgm.New(cgmRepo, glucoseRepo, &stubEncryptor{}, cgm.WithLibreLinkUpBaseURL(srv.URL))

	env := newTestEnvWithCGM(t, cgmUC)

	// Go through the LLU flow.
	env.sendCallback("profile:cgm")
	env.sendCallback("cgm:provider:librelinkup")
	env.sendCallback("cgm:llu:connect")
	env.sendText("test@example.com")
	env.bot.reset()
	env.sendText("wrongpassword")

	// Should show an auth error from the mock.
	msg := env.bot.lastMessageText()
	assert.Contains(t, msg, "авторизации")

	// No connection should be saved.
	cnt := env.queryInt(
		`SELECT count(*) FROM cgm_connections WHERE user_id = $1`,
		env.userID,
	)
	assert.Equal(t, int64(0), cnt)
}

func TestCGMLibreLinkUpInvalidEmail(t *testing.T) {
	glucoseRepo := postgresrepo.NewGlucoseRepo(testPool)
	cgmRepo := postgresrepo.NewCGMConnectionRepo(testPool)
	cgmUC := cgm.New(cgmRepo, glucoseRepo, &stubEncryptor{})

	env := newTestEnvWithCGM(t, cgmUC)

	env.sendCallback("profile:cgm")
	env.sendCallback("cgm:provider:librelinkup")
	env.sendCallback("cgm:llu:connect")

	// Enter invalid email (no @).
	env.bot.reset()
	env.sendText("notanemail")
	msg := env.bot.lastMessageText()
	assert.Contains(t, msg, "email")
}
