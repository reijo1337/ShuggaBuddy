package telegram_test

import (
	"context"
	"testing"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"

	"github.com/gmtantsevov/shuggabuddy/internal/delivery/telegram"
	tgmocks "github.com/gmtantsevov/shuggabuddy/internal/delivery/telegram/mocks"
	"github.com/gmtantsevov/shuggabuddy/internal/domain"
	"github.com/gmtantsevov/shuggabuddy/internal/i18n"
	"github.com/gmtantsevov/shuggabuddy/pkg/librelinkup"
	"github.com/gmtantsevov/shuggabuddy/pkg/nightscout"
)

func newCGMTestHandler(
	bot *spyBot,
	userUC telegram.UserUseCase,
	cgmUC telegram.CGMUseCase,
	loc *i18n.Localizer,
) *telegram.Handler {
	return telegram.NewHandler(bot, userUC, nil, nil, nil, nil, nil, nil, nil, nil, cgmUC, loc, zap.NewNop())
}

// --- CGM: provider selection (not connected) ---

func TestCGMShow_NotConnected_ShowsProviderSelection(t *testing.T) {
	ctrl := gomock.NewController(t)
	bot := &spyBot{}
	userUC := tgmocks.NewMockUserUseCase(ctrl)
	cgmUC := tgmocks.NewMockCGMUseCase(ctrl)
	loc := newTestLocalizer(t)

	userUC.EXPECT().
		GetProfile(gomock.Any(), domain.ProviderTelegram, "123").
		Return(testUser, testAcc, nil)
	cgmUC.EXPECT().
		GetConnection(gomock.Any(), int64(1)).
		Return(nil, nil)

	h := newCGMTestHandler(bot, userUC, cgmUC, loc)
	h.HandleCallback(context.Background(), testCallback(123, 456, "profile:cgm"))

	require.Len(t, bot.sent, 1)
	sent := bot.lastMessage()
	assert.Contains(t, sent.Text, "CGM")
	assert.Contains(t, sent.Text, "Подключи сенсор")

	kb := sent.ReplyMarkup.(tgbotapi.InlineKeyboardMarkup)
	assert.Equal(t, "cgm:provider:nightscout", *kb.InlineKeyboard[0][0].CallbackData)
	assert.Equal(t, "cgm:provider:librelinkup", *kb.InlineKeyboard[0][1].CallbackData)
}

// --- CGM: connected status with provider name ---

func TestCGMShow_Connected_Nightscout(t *testing.T) {
	ctrl := gomock.NewController(t)
	bot := &spyBot{}
	userUC := tgmocks.NewMockUserUseCase(ctrl)
	cgmUC := tgmocks.NewMockCGMUseCase(ctrl)
	loc := newTestLocalizer(t)

	syncTime := time.Date(2026, 4, 19, 10, 0, 0, 0, time.UTC)
	conn := &domain.CGMConnection{
		Provider:     domain.CGMProviderNightscout,
		BaseURL:      "https://my.nightscout.com",
		Active:       true,
		LastSyncedAt: &syncTime,
	}

	userUC.EXPECT().
		GetProfile(gomock.Any(), domain.ProviderTelegram, "123").
		Return(testUser, testAcc, nil)
	cgmUC.EXPECT().
		GetConnection(gomock.Any(), int64(1)).
		Return(conn, nil)

	h := newCGMTestHandler(bot, userUC, cgmUC, loc)
	h.HandleCallback(context.Background(), testCallback(123, 456, "profile:cgm"))

	require.Len(t, bot.sent, 1)
	sent := bot.lastMessage()
	assert.Contains(t, sent.Text, "Nightscout")
	assert.Contains(t, sent.Text, "https://my.nightscout.com")
}

func TestCGMShow_Connected_LibreLinkUp(t *testing.T) {
	ctrl := gomock.NewController(t)
	bot := &spyBot{}
	userUC := tgmocks.NewMockUserUseCase(ctrl)
	cgmUC := tgmocks.NewMockCGMUseCase(ctrl)
	loc := newTestLocalizer(t)

	syncTime := time.Date(2026, 4, 19, 10, 0, 0, 0, time.UTC)
	conn := &domain.CGMConnection{
		Provider:     domain.CGMProviderLibreLinkUp,
		BaseURL:      "https://api.libreview.io",
		Active:       true,
		LastSyncedAt: &syncTime,
	}

	userUC.EXPECT().
		GetProfile(gomock.Any(), domain.ProviderTelegram, "123").
		Return(testUser, testAcc, nil)
	cgmUC.EXPECT().
		GetConnection(gomock.Any(), int64(1)).
		Return(conn, nil)

	h := newCGMTestHandler(bot, userUC, cgmUC, loc)
	h.HandleCallback(context.Background(), testCallback(123, 456, "profile:cgm"))

	require.Len(t, bot.sent, 1)
	sent := bot.lastMessage()
	assert.Contains(t, sent.Text, "LibreLinkUp")
}

// --- CGM: Nightscout provider intro ---

func TestCGMNightscoutIntro(t *testing.T) {
	ctrl := gomock.NewController(t)
	bot := &spyBot{}
	userUC := tgmocks.NewMockUserUseCase(ctrl)
	cgmUC := tgmocks.NewMockCGMUseCase(ctrl)
	loc := newTestLocalizer(t)

	userUC.EXPECT().
		GetProfile(gomock.Any(), domain.ProviderTelegram, "123").
		Return(testUser, testAcc, nil)

	h := newCGMTestHandler(bot, userUC, cgmUC, loc)
	h.HandleCallback(context.Background(), testCallback(123, 456, "cgm:provider:nightscout"))

	require.Len(t, bot.sent, 1)
	sent := bot.lastMessage()
	assert.Contains(t, sent.Text, "Nightscout")

	kb := sent.ReplyMarkup.(tgbotapi.InlineKeyboardMarkup)
	assert.Equal(t, "cgm:connect", *kb.InlineKeyboard[0][0].CallbackData)
	assert.Equal(t, "cgm:back", *kb.InlineKeyboard[1][0].CallbackData)
}

// --- CGM: LLU provider intro ---

func TestCGMLLUIntro(t *testing.T) {
	ctrl := gomock.NewController(t)
	bot := &spyBot{}
	userUC := tgmocks.NewMockUserUseCase(ctrl)
	cgmUC := tgmocks.NewMockCGMUseCase(ctrl)
	loc := newTestLocalizer(t)

	userUC.EXPECT().
		GetProfile(gomock.Any(), domain.ProviderTelegram, "123").
		Return(testUser, testAcc, nil)

	h := newCGMTestHandler(bot, userUC, cgmUC, loc)
	h.HandleCallback(context.Background(), testCallback(123, 456, "cgm:provider:librelinkup"))

	require.Len(t, bot.sent, 1)
	sent := bot.lastMessage()
	assert.Contains(t, sent.Text, "LibreLinkUp")

	kb := sent.ReplyMarkup.(tgbotapi.InlineKeyboardMarkup)
	assert.Equal(t, "cgm:llu:connect", *kb.InlineKeyboard[0][0].CallbackData)
	assert.Equal(t, "cgm:back", *kb.InlineKeyboard[1][0].CallbackData)
}

// --- CGM: Nightscout connect flow ---

func TestCGMNightscoutConnect_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	bot := &spyBot{}
	userUC := tgmocks.NewMockUserUseCase(ctrl)
	cgmUC := tgmocks.NewMockCGMUseCase(ctrl)
	loc := newTestLocalizer(t)

	userUC.EXPECT().
		GetProfile(gomock.Any(), domain.ProviderTelegram, "123").
		Return(testUser, testAcc, nil).Times(2)
	cgmUC.EXPECT().
		AddConnection(gomock.Any(), int64(1), domain.CGMProviderNightscout, "https://my.nightscout.com", "secret123").
		Return(nil)

	h := newCGMTestHandler(bot, userUC, cgmUC, loc)

	// Start connect flow
	h.HandleCallback(context.Background(), testCallback(123, 456, "cgm:connect"))
	require.Len(t, bot.sent, 1)
	assert.Contains(t, bot.lastMessage().Text, "URL")

	// Enter URL
	sess := telegram.NewSession(telegram.SessionCGM, telegram.StepCGMURL)
	h.HandleSessionInput(context.Background(), testMessage(123, 456, "https://my.nightscout.com"), sess)
	require.Len(t, bot.sent, 2)
	assert.Contains(t, bot.sent[1].(tgbotapi.MessageConfig).Text, "API Secret")

	// Enter token
	sess.Step = telegram.StepCGMToken
	h.HandleSessionInput(context.Background(), testMessage(123, 456, "secret123"), sess)
	require.Len(t, bot.sent, 3)
	assert.Contains(t, bot.sent[2].(tgbotapi.MessageConfig).Text, "Nightscout подключён")
}

func TestCGMNightscoutConnect_AuthError(t *testing.T) {
	ctrl := gomock.NewController(t)
	bot := &spyBot{}
	userUC := tgmocks.NewMockUserUseCase(ctrl)
	cgmUC := tgmocks.NewMockCGMUseCase(ctrl)
	loc := newTestLocalizer(t)

	userUC.EXPECT().
		GetProfile(gomock.Any(), domain.ProviderTelegram, "123").
		Return(testUser, testAcc, nil)
	cgmUC.EXPECT().
		AddConnection(gomock.Any(), int64(1), domain.CGMProviderNightscout, "https://my.nightscout.com", "bad").
		Return(nightscout.ErrUnauthorized)

	h := newCGMTestHandler(bot, userUC, cgmUC, loc)

	sess := telegram.NewSession(telegram.SessionCGM, telegram.StepCGMToken)
	sess.Data["url"] = "https://my.nightscout.com"
	h.HandleSessionInput(context.Background(), testMessage(123, 456, "bad"), sess)

	require.Len(t, bot.sent, 1)
	assert.Contains(t, bot.lastMessage().Text, "авторизации")
}

func TestCGMNightscoutConnect_InvalidURL(t *testing.T) {
	bot := &spyBot{}
	ctrl := gomock.NewController(t)
	userUC := tgmocks.NewMockUserUseCase(ctrl)
	cgmUC := tgmocks.NewMockCGMUseCase(ctrl)
	loc := newTestLocalizer(t)

	h := newCGMTestHandler(bot, userUC, cgmUC, loc)

	sess := telegram.NewSession(telegram.SessionCGM, telegram.StepCGMURL)
	h.HandleSessionInput(context.Background(), testMessage(123, 456, "not-a-url"), sess)

	require.Len(t, bot.sent, 1)
	assert.Contains(t, bot.lastMessage().Text, "Некорректный URL")
}

// --- CGM: LLU connect flow ---

func TestCGMLLUConnect_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	bot := &spyBot{}
	userUC := tgmocks.NewMockUserUseCase(ctrl)
	cgmUC := tgmocks.NewMockCGMUseCase(ctrl)
	loc := newTestLocalizer(t)

	userUC.EXPECT().
		GetProfile(gomock.Any(), domain.ProviderTelegram, "123").
		Return(testUser, testAcc, nil).Times(2)
	cgmUC.EXPECT().
		AddConnection(gomock.Any(), int64(1), domain.CGMProviderLibreLinkUp, "test@example.com", "password123").
		Return(nil)

	h := newCGMTestHandler(bot, userUC, cgmUC, loc)

	// Start LLU connect flow
	h.HandleCallback(context.Background(), testCallback(123, 456, "cgm:llu:connect"))
	require.Len(t, bot.sent, 1)
	assert.Contains(t, bot.lastMessage().Text, "email")

	// Enter email
	sess := telegram.NewSession(telegram.SessionCGM, telegram.StepLLUEmail)
	h.HandleSessionInput(context.Background(), testMessage(123, 456, "test@example.com"), sess)
	require.Len(t, bot.sent, 2)
	assert.Contains(t, bot.sent[1].(tgbotapi.MessageConfig).Text, "пароль")

	// Enter password
	h.HandleSessionInput(context.Background(), testMessage(123, 456, "password123"), sess)
	require.Len(t, bot.sent, 3)
	assert.Contains(t, bot.sent[2].(tgbotapi.MessageConfig).Text, "LibreLinkUp подключён")
}

func TestCGMLLUConnect_InvalidEmail(t *testing.T) {
	bot := &spyBot{}
	ctrl := gomock.NewController(t)
	userUC := tgmocks.NewMockUserUseCase(ctrl)
	cgmUC := tgmocks.NewMockCGMUseCase(ctrl)
	loc := newTestLocalizer(t)

	h := newCGMTestHandler(bot, userUC, cgmUC, loc)

	sess := telegram.NewSession(telegram.SessionCGM, telegram.StepLLUEmail)
	h.HandleSessionInput(context.Background(), testMessage(123, 456, "notanemail"), sess)

	require.Len(t, bot.sent, 1)
	assert.Contains(t, bot.lastMessage().Text, "Некорректный email")
}

func TestCGMLLUConnect_AuthFail(t *testing.T) {
	ctrl := gomock.NewController(t)
	bot := &spyBot{}
	userUC := tgmocks.NewMockUserUseCase(ctrl)
	cgmUC := tgmocks.NewMockCGMUseCase(ctrl)
	loc := newTestLocalizer(t)

	userUC.EXPECT().
		GetProfile(gomock.Any(), domain.ProviderTelegram, "123").
		Return(testUser, testAcc, nil)
	cgmUC.EXPECT().
		AddConnection(gomock.Any(), int64(1), domain.CGMProviderLibreLinkUp, "test@example.com", "wrong").
		Return(librelinkup.ErrUnauthorized)

	h := newCGMTestHandler(bot, userUC, cgmUC, loc)

	sess := telegram.NewSession(telegram.SessionCGM, telegram.StepLLUPassword)
	sess.Data["email"] = "test@example.com"
	h.HandleSessionInput(context.Background(), testMessage(123, 456, "wrong"), sess)

	require.Len(t, bot.sent, 1)
	assert.Contains(t, bot.lastMessage().Text, "авторизации")
	assert.Contains(t, bot.lastMessage().Text, "LibreView")
}

func TestCGMLLUConnect_NoPatients(t *testing.T) {
	ctrl := gomock.NewController(t)
	bot := &spyBot{}
	userUC := tgmocks.NewMockUserUseCase(ctrl)
	cgmUC := tgmocks.NewMockCGMUseCase(ctrl)
	loc := newTestLocalizer(t)

	userUC.EXPECT().
		GetProfile(gomock.Any(), domain.ProviderTelegram, "123").
		Return(testUser, testAcc, nil)
	cgmUC.EXPECT().
		AddConnection(gomock.Any(), int64(1), domain.CGMProviderLibreLinkUp, "test@example.com", "pass").
		Return(librelinkup.ErrNoPatients)

	h := newCGMTestHandler(bot, userUC, cgmUC, loc)

	sess := telegram.NewSession(telegram.SessionCGM, telegram.StepLLUPassword)
	sess.Data["email"] = "test@example.com"
	h.HandleSessionInput(context.Background(), testMessage(123, 456, "pass"), sess)

	require.Len(t, bot.sent, 1)
	assert.Contains(t, bot.lastMessage().Text, "не найдены подключённые пациенты")
}

// --- CGM: disconnect ---

func TestCGMDisconnect(t *testing.T) {
	ctrl := gomock.NewController(t)
	bot := &spyBot{}
	userUC := tgmocks.NewMockUserUseCase(ctrl)
	cgmUC := tgmocks.NewMockCGMUseCase(ctrl)
	loc := newTestLocalizer(t)

	userUC.EXPECT().
		GetProfile(gomock.Any(), domain.ProviderTelegram, "123").
		Return(testUser, testAcc, nil)
	cgmUC.EXPECT().
		RemoveConnection(gomock.Any(), int64(1)).
		Return(nil)

	h := newCGMTestHandler(bot, userUC, cgmUC, loc)
	h.HandleCallback(context.Background(), testCallback(123, 456, "cgm:disconnect:yes"))

	require.Len(t, bot.sent, 1)
	assert.Contains(t, bot.lastMessage().Text, "CGM отключён")
}

func TestCGMDisconnectConfirm(t *testing.T) {
	ctrl := gomock.NewController(t)
	bot := &spyBot{}
	userUC := tgmocks.NewMockUserUseCase(ctrl)
	cgmUC := tgmocks.NewMockCGMUseCase(ctrl)
	loc := newTestLocalizer(t)

	userUC.EXPECT().
		GetProfile(gomock.Any(), domain.ProviderTelegram, "123").
		Return(testUser, testAcc, nil)

	h := newCGMTestHandler(bot, userUC, cgmUC, loc)
	h.HandleCallback(context.Background(), testCallback(123, 456, "cgm:disconnect"))

	require.Len(t, bot.sent, 1)
	assert.Contains(t, bot.lastMessage().Text, "Отключить CGM")
}

// --- CGM: test ---

func TestCGMTest_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	bot := &spyBot{}
	userUC := tgmocks.NewMockUserUseCase(ctrl)
	cgmUC := tgmocks.NewMockCGMUseCase(ctrl)
	loc := newTestLocalizer(t)

	userUC.EXPECT().
		GetProfile(gomock.Any(), domain.ProviderTelegram, "123").
		Return(testUser, testAcc, nil)
	cgmUC.EXPECT().
		TestConnection(gomock.Any(), int64(1)).
		Return(nil)

	h := newCGMTestHandler(bot, userUC, cgmUC, loc)
	h.HandleCallback(context.Background(), testCallback(123, 456, "cgm:test"))

	require.Len(t, bot.sent, 1)
	assert.Contains(t, bot.lastMessage().Text, "работает")
}

// --- CGM: no encryption key ---

func TestCGMShow_NoEncryptionKey(t *testing.T) {
	ctrl := gomock.NewController(t)
	bot := &spyBot{}
	userUC := tgmocks.NewMockUserUseCase(ctrl)
	loc := newTestLocalizer(t)

	userUC.EXPECT().
		GetProfile(gomock.Any(), domain.ProviderTelegram, "123").
		Return(testUser, testAcc, nil)

	// Pass nil as cgmUC
	h := newCGMTestHandler(bot, userUC, nil, loc)
	h.HandleCallback(context.Background(), testCallback(123, 456, "profile:cgm"))

	require.Len(t, bot.sent, 1)
	assert.Contains(t, bot.lastMessage().Text, "временно недоступна")
}
