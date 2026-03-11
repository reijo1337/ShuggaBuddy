package telegram_test

import (
	"context"
	"errors"
	"testing"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"

	"github.com/gmtantsevov/shuggabuddy/internal/delivery/telegram"
	"github.com/gmtantsevov/shuggabuddy/internal/delivery/telegram/mocks"
	"github.com/gmtantsevov/shuggabuddy/internal/domain"
	"github.com/gmtantsevov/shuggabuddy/internal/i18n"
)

// spyBot перехватывает вызовы Send и Request для проверки в тестах.
type spyBot struct {
	sent     []tgbotapi.Chattable
	requests []tgbotapi.Chattable
}

func (s *spyBot) Send(c tgbotapi.Chattable) (tgbotapi.Message, error) {
	s.sent = append(s.sent, c)
	return tgbotapi.Message{}, nil
}

func (s *spyBot) Request(c tgbotapi.Chattable) (*tgbotapi.APIResponse, error) {
	s.requests = append(s.requests, c)
	return &tgbotapi.APIResponse{Ok: true}, nil
}

func (s *spyBot) GetUpdatesChan(_ tgbotapi.UpdateConfig) tgbotapi.UpdatesChannel {
	return make(tgbotapi.UpdatesChannel)
}

func (s *spyBot) StopReceivingUpdates() {}

func (s *spyBot) lastMessage() tgbotapi.MessageConfig {
	return s.sent[len(s.sent)-1].(tgbotapi.MessageConfig)
}

func newTestLocalizer(t *testing.T) *i18n.Localizer {
	t.Helper()
	loc, err := i18n.NewLocalizer("../../../locales", "ru")
	require.NoError(t, err)
	return loc
}

func newTestHandler(
	bot *spyBot,
	userUC telegram.UserUseCase,
	glucUC telegram.GlucoseUseCase,
	loc *i18n.Localizer,
) *telegram.Handler {
	return telegram.NewHandler(bot, userUC, glucUC, loc, zap.NewNop())
}

func testMessage(userID int64, chatID int64, text string) *tgbotapi.Message {
	return &tgbotapi.Message{
		From: &tgbotapi.User{ID: userID, FirstName: "Test"},
		Chat: &tgbotapi.Chat{ID: chatID},
		Text: text,
	}
}

func testCallback(userID int64, chatID int64, data string) *tgbotapi.CallbackQuery {
	return &tgbotapi.CallbackQuery{
		ID:   "cb_1",
		From: &tgbotapi.User{ID: userID, FirstName: "Test"},
		Message: &tgbotapi.Message{
			Chat: &tgbotapi.Chat{ID: chatID},
		},
		Data: data,
	}
}

var testUser = &domain.User{
	ID:        1,
	Units:     domain.UnitsMmol,
	CreatedAt: time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC),
}

var testAcc = &domain.ExternalAccount{
	ID:          1,
	UserID:      1,
	Provider:    domain.ProviderTelegram,
	ExternalID:  "123",
	DisplayName: "Test",
}

// --- /start ---

func TestHandleStart_NewUser(t *testing.T) {
	ctrl := gomock.NewController(t)
	bot := &spyBot{}
	userUC := tgmocks.NewMockUserUseCase(ctrl)
	glucUC := tgmocks.NewMockGlucoseUseCase(ctrl)
	loc := newTestLocalizer(t)

	userUC.EXPECT().
		GetOrCreateUser(gomock.Any(), domain.ProviderTelegram, "123", "Test").
		Return(testUser, testAcc, true, nil)

	h := newTestHandler(bot, userUC, glucUC, loc)

	msg := testMessage(123, 456, "/start")
	msg.Entities = []tgbotapi.MessageEntity{{Type: "bot_command", Length: 6}}
	h.HandleStart(context.Background(), msg)

	require.Len(t, bot.sent, 1)
	sent := bot.lastMessage()
	assert.Equal(t, int64(456), sent.ChatID)
	assert.Contains(t, sent.Text, "Привет, Test!")
	assert.Contains(t, sent.Text, "Выбери действие:")
	assert.NotNil(t, sent.ReplyMarkup)
}

func TestHandleStart_ExistingUser(t *testing.T) {
	ctrl := gomock.NewController(t)
	bot := &spyBot{}
	userUC := tgmocks.NewMockUserUseCase(ctrl)
	glucUC := tgmocks.NewMockGlucoseUseCase(ctrl)
	loc := newTestLocalizer(t)

	userUC.EXPECT().
		GetOrCreateUser(gomock.Any(), domain.ProviderTelegram, "123", "Test").
		Return(testUser, testAcc, false, nil)

	h := newTestHandler(bot, userUC, glucUC, loc)

	msg := testMessage(123, 456, "/start")
	msg.Entities = []tgbotapi.MessageEntity{{Type: "bot_command", Length: 6}}
	h.HandleStart(context.Background(), msg)

	require.Len(t, bot.sent, 1)
	sent := bot.lastMessage()
	assert.Contains(t, sent.Text, "С возвращением, Test!")
	assert.Contains(t, sent.Text, "Выбери действие:")
}

func TestHandleStart_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	bot := &spyBot{}
	userUC := tgmocks.NewMockUserUseCase(ctrl)
	glucUC := tgmocks.NewMockGlucoseUseCase(ctrl)
	loc := newTestLocalizer(t)

	userUC.EXPECT().
		GetOrCreateUser(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, nil, false, errors.New("db error"))

	h := newTestHandler(bot, userUC, glucUC, loc)

	msg := testMessage(123, 456, "/start")
	msg.Entities = []tgbotapi.MessageEntity{{Type: "bot_command", Length: 6}}
	h.HandleStart(context.Background(), msg)

	require.Len(t, bot.sent, 1)
	assert.Contains(t, bot.lastMessage().Text, "внутренняя ошибка")
}

// --- Callback: menu:profile ---

func TestHandleCallback_Profile(t *testing.T) {
	ctrl := gomock.NewController(t)
	bot := &spyBot{}
	userUC := tgmocks.NewMockUserUseCase(ctrl)
	glucUC := tgmocks.NewMockGlucoseUseCase(ctrl)
	loc := newTestLocalizer(t)

	userUC.EXPECT().
		GetProfile(gomock.Any(), domain.ProviderTelegram, "123").
		Return(testUser, testAcc, nil)

	h := newTestHandler(bot, userUC, glucUC, loc)
	h.HandleCallback(context.Background(), testCallback(123, 456, "menu:profile"))

	require.Len(t, bot.sent, 1)
	sent := bot.lastMessage()
	assert.Contains(t, sent.Text, "Test")
	assert.Contains(t, sent.Text, "ммоль/л")
	assert.Contains(t, sent.Text, "15.01.2025")
}

func TestHandleCallback_Profile_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	bot := &spyBot{}
	userUC := tgmocks.NewMockUserUseCase(ctrl)
	glucUC := tgmocks.NewMockGlucoseUseCase(ctrl)
	loc := newTestLocalizer(t)

	userUC.EXPECT().
		GetProfile(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, nil, errors.New("db error"))

	h := newTestHandler(bot, userUC, glucUC, loc)
	h.HandleCallback(context.Background(), testCallback(123, 456, "menu:profile"))

	require.Len(t, bot.sent, 1)
	assert.Contains(t, bot.lastMessage().Text, "внутренняя ошибка")
}

// --- Callback: menu:units ---

func TestHandleCallback_SetUnits(t *testing.T) {
	bot := &spyBot{}
	ctrl := gomock.NewController(t)
	userUC := tgmocks.NewMockUserUseCase(ctrl)
	glucUC := tgmocks.NewMockGlucoseUseCase(ctrl)
	loc := newTestLocalizer(t)

	h := newTestHandler(bot, userUC, glucUC, loc)
	h.HandleCallback(context.Background(), testCallback(123, 456, "menu:units"))

	require.Len(t, bot.sent, 1)
	sent := bot.lastMessage()
	assert.Contains(t, sent.Text, "Выбери единицы измерения:")
	kb := sent.ReplyMarkup.(tgbotapi.InlineKeyboardMarkup)
	assert.Len(t, kb.InlineKeyboard, 2)
	assert.Equal(t, "units:mmol", *kb.InlineKeyboard[0][0].CallbackData)
	assert.Equal(t, "units:mgdl", *kb.InlineKeyboard[0][1].CallbackData)
	assert.Equal(t, "menu:back", *kb.InlineKeyboard[1][0].CallbackData)
}

// --- Callback: units:mmol / units:mgdl ---

func TestHandleCallback_SetUnitsMmol(t *testing.T) {
	ctrl := gomock.NewController(t)
	bot := &spyBot{}
	userUC := tgmocks.NewMockUserUseCase(ctrl)
	glucUC := tgmocks.NewMockGlucoseUseCase(ctrl)
	loc := newTestLocalizer(t)

	userUC.EXPECT().
		GetProfile(gomock.Any(), domain.ProviderTelegram, "123").
		Return(testUser, testAcc, nil)
	userUC.EXPECT().
		UpdateUnits(gomock.Any(), int64(1), domain.UnitsMmol).
		Return(nil)

	h := newTestHandler(bot, userUC, glucUC, loc)
	h.HandleCallback(context.Background(), testCallback(123, 456, "units:mmol"))

	require.Len(t, bot.sent, 1)
	sent := bot.lastMessage()
	assert.Contains(t, sent.Text, "ммоль/л")
	assert.Contains(t, sent.Text, "Выбери действие:")
}

func TestHandleCallback_SetUnitsMgdl(t *testing.T) {
	ctrl := gomock.NewController(t)
	bot := &spyBot{}
	userUC := tgmocks.NewMockUserUseCase(ctrl)
	glucUC := tgmocks.NewMockGlucoseUseCase(ctrl)
	loc := newTestLocalizer(t)

	userUC.EXPECT().
		GetProfile(gomock.Any(), domain.ProviderTelegram, "123").
		Return(testUser, testAcc, nil)
	userUC.EXPECT().
		UpdateUnits(gomock.Any(), int64(1), domain.UnitsMgdl).
		Return(nil)

	h := newTestHandler(bot, userUC, glucUC, loc)
	h.HandleCallback(context.Background(), testCallback(123, 456, "units:mgdl"))

	require.Len(t, bot.sent, 1)
	sent := bot.lastMessage()
	assert.Contains(t, sent.Text, "мг/дл")
	assert.Contains(t, sent.Text, "Выбери действие:")
}

func TestHandleCallback_SetUnits_UpdateError(t *testing.T) {
	ctrl := gomock.NewController(t)
	bot := &spyBot{}
	userUC := tgmocks.NewMockUserUseCase(ctrl)
	glucUC := tgmocks.NewMockGlucoseUseCase(ctrl)
	loc := newTestLocalizer(t)

	userUC.EXPECT().
		GetProfile(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(testUser, testAcc, nil)
	userUC.EXPECT().
		UpdateUnits(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(errors.New("db error"))

	h := newTestHandler(bot, userUC, glucUC, loc)
	h.HandleCallback(context.Background(), testCallback(123, 456, "units:mmol"))

	require.Len(t, bot.sent, 1)
	assert.Contains(t, bot.lastMessage().Text, "внутренняя ошибка")
}

// --- Callback: menu:glucose ---

func TestHandleCallback_GlucoseStart(t *testing.T) {
	bot := &spyBot{}
	ctrl := gomock.NewController(t)
	userUC := tgmocks.NewMockUserUseCase(ctrl)
	glucUC := tgmocks.NewMockGlucoseUseCase(ctrl)
	loc := newTestLocalizer(t)

	h := newTestHandler(bot, userUC, glucUC, loc)
	h.HandleCallback(context.Background(), testCallback(123, 456, "menu:glucose"))

	require.Len(t, bot.sent, 1)
	sent := bot.lastMessage()
	assert.Contains(t, sent.Text, "Введи уровень сахара:")
	kb := sent.ReplyMarkup.(tgbotapi.InlineKeyboardMarkup)
	assert.Equal(t, "menu:back", *kb.InlineKeyboard[0][0].CallbackData)
}

// --- Glucose input ---

func TestHandleGlucoseInput_Valid(t *testing.T) {
	ctrl := gomock.NewController(t)
	bot := &spyBot{}
	userUC := tgmocks.NewMockUserUseCase(ctrl)
	glucUC := tgmocks.NewMockGlucoseUseCase(ctrl)
	loc := newTestLocalizer(t)

	userUC.EXPECT().
		GetProfile(gomock.Any(), domain.ProviderTelegram, "123").
		Return(testUser, testAcc, nil)
	glucUC.EXPECT().
		SaveReading(gomock.Any(), int64(1), 5.4, domain.UnitsMmol).
		Return(nil)

	h := newTestHandler(bot, userUC, glucUC, loc)
	h.HandleGlucoseInput(context.Background(), testMessage(123, 456, "5.4"))

	require.Len(t, bot.sent, 1)
	sent := bot.lastMessage()
	assert.Contains(t, sent.Text, "Записано:")
	assert.Contains(t, sent.Text, "5.4")
	assert.Contains(t, sent.Text, "ммоль/л")
}

func TestHandleGlucoseInput_InvalidText(t *testing.T) {
	bot := &spyBot{}
	ctrl := gomock.NewController(t)
	userUC := tgmocks.NewMockUserUseCase(ctrl)
	glucUC := tgmocks.NewMockGlucoseUseCase(ctrl)
	loc := newTestLocalizer(t)

	h := newTestHandler(bot, userUC, glucUC, loc)
	h.HandleGlucoseInput(context.Background(), testMessage(123, 456, "abc"))

	require.Len(t, bot.sent, 1)
	assert.Contains(t, bot.lastMessage().Text, "Некорректное значение")
}

func TestHandleGlucoseInput_OutOfRange(t *testing.T) {
	ctrl := gomock.NewController(t)
	bot := &spyBot{}
	userUC := tgmocks.NewMockUserUseCase(ctrl)
	glucUC := tgmocks.NewMockGlucoseUseCase(ctrl)
	loc := newTestLocalizer(t)

	userUC.EXPECT().
		GetProfile(gomock.Any(), domain.ProviderTelegram, "123").
		Return(testUser, testAcc, nil)
	glucUC.EXPECT().
		SaveReading(gomock.Any(), int64(1), 50.0, domain.UnitsMmol).
		Return(errors.New("out of range"))

	h := newTestHandler(bot, userUC, glucUC, loc)
	h.HandleGlucoseInput(context.Background(), testMessage(123, 456, "50.0"))

	require.Len(t, bot.sent, 1)
	assert.Contains(t, bot.lastMessage().Text, "вне допустимого диапазона")
}

func TestHandleGlucoseInput_OutOfRange_Mgdl(t *testing.T) {
	ctrl := gomock.NewController(t)
	bot := &spyBot{}
	userUC := tgmocks.NewMockUserUseCase(ctrl)
	glucUC := tgmocks.NewMockGlucoseUseCase(ctrl)
	loc := newTestLocalizer(t)

	mgdlUser := &domain.User{ID: 1, Units: domain.UnitsMgdl}

	userUC.EXPECT().
		GetProfile(gomock.Any(), domain.ProviderTelegram, "123").
		Return(mgdlUser, testAcc, nil)
	glucUC.EXPECT().
		SaveReading(gomock.Any(), int64(1), 700.0, domain.UnitsMgdl).
		Return(errors.New("out of range"))

	h := newTestHandler(bot, userUC, glucUC, loc)
	h.HandleGlucoseInput(context.Background(), testMessage(123, 456, "700"))

	require.Len(t, bot.sent, 1)
	assert.Contains(t, bot.lastMessage().Text, "мг/дл")
}

// --- Callback: menu:last ---

func TestHandleCallback_Last(t *testing.T) {
	ctrl := gomock.NewController(t)
	bot := &spyBot{}
	userUC := tgmocks.NewMockUserUseCase(ctrl)
	glucUC := tgmocks.NewMockGlucoseUseCase(ctrl)
	loc := newTestLocalizer(t)

	readings := []domain.GlucoseReading{
		{ID: 1, UserID: 1, ValueMmol: 5.4, RecordedAt: time.Date(2025, 3, 10, 8, 0, 0, 0, time.UTC)},
		{ID: 2, UserID: 1, ValueMmol: 6.1, RecordedAt: time.Date(2025, 3, 10, 12, 0, 0, 0, time.UTC)},
	}

	userUC.EXPECT().
		GetProfile(gomock.Any(), domain.ProviderTelegram, "123").
		Return(testUser, testAcc, nil)
	glucUC.EXPECT().
		GetLastReadings(gomock.Any(), int64(1), 5).
		Return(readings, nil)

	h := newTestHandler(bot, userUC, glucUC, loc)
	h.HandleCallback(context.Background(), testCallback(123, 456, "menu:last"))

	require.Len(t, bot.sent, 1)
	sent := bot.lastMessage()
	assert.Contains(t, sent.Text, "Последние записи:")
	assert.Contains(t, sent.Text, "5.4")
	assert.Contains(t, sent.Text, "6.1")
}

func TestHandleCallback_Last_Empty(t *testing.T) {
	ctrl := gomock.NewController(t)
	bot := &spyBot{}
	userUC := tgmocks.NewMockUserUseCase(ctrl)
	glucUC := tgmocks.NewMockGlucoseUseCase(ctrl)
	loc := newTestLocalizer(t)

	userUC.EXPECT().
		GetProfile(gomock.Any(), domain.ProviderTelegram, "123").
		Return(testUser, testAcc, nil)
	glucUC.EXPECT().
		GetLastReadings(gomock.Any(), int64(1), 5).
		Return(nil, nil)

	h := newTestHandler(bot, userUC, glucUC, loc)
	h.HandleCallback(context.Background(), testCallback(123, 456, "menu:last"))

	require.Len(t, bot.sent, 1)
	assert.Contains(t, bot.lastMessage().Text, "Записей пока нет")
}

// --- Callback: menu:back ---

func TestHandleCallback_Back(t *testing.T) {
	ctrl := gomock.NewController(t)
	bot := &spyBot{}
	userUC := tgmocks.NewMockUserUseCase(ctrl)
	glucUC := tgmocks.NewMockGlucoseUseCase(ctrl)
	loc := newTestLocalizer(t)

	userUC.EXPECT().
		GetProfile(gomock.Any(), domain.ProviderTelegram, "123").
		Return(testUser, testAcc, nil)

	h := newTestHandler(bot, userUC, glucUC, loc)
	h.HandleCallback(context.Background(), testCallback(123, 456, "menu:back"))

	require.Len(t, bot.sent, 1)
	sent := bot.lastMessage()
	assert.Contains(t, sent.Text, "Выбери действие:")
	kb := sent.ReplyMarkup.(tgbotapi.InlineKeyboardMarkup)
	assert.Len(t, kb.InlineKeyboard, 4)
}

// --- Waiting glucose state ---

func TestHandleCallback_Back_ClearsWaitingState(t *testing.T) {
	ctrl := gomock.NewController(t)
	bot := &spyBot{}
	userUC := tgmocks.NewMockUserUseCase(ctrl)
	glucUC := tgmocks.NewMockGlucoseUseCase(ctrl)
	loc := newTestLocalizer(t)

	userUC.EXPECT().
		GetProfile(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(testUser, testAcc, nil).
		Times(1)

	h := newTestHandler(bot, userUC, glucUC, loc)

	// Активируем ожидание ввода глюкозы.
	h.HandleCallback(context.Background(), testCallback(123, 456, "menu:glucose"))
	require.Len(t, bot.sent, 1)

	// Нажимаем "В меню" — состояние ожидания должно очиститься.
	h.HandleCallback(context.Background(), testCallback(123, 456, "menu:back"))
	require.Len(t, bot.sent, 2)
	assert.Contains(t, bot.sent[1].(tgbotapi.MessageConfig).Text, "Выбери действие:")
}

// --- Menu keyboard has correct buttons ---

func TestMenuKeyboard_ContainsCurrentUnits(t *testing.T) {
	ctrl := gomock.NewController(t)
	bot := &spyBot{}
	userUC := tgmocks.NewMockUserUseCase(ctrl)
	glucUC := tgmocks.NewMockGlucoseUseCase(ctrl)
	loc := newTestLocalizer(t)

	mgdlUser := &domain.User{ID: 1, Units: domain.UnitsMgdl}

	userUC.EXPECT().
		GetOrCreateUser(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(mgdlUser, testAcc, false, nil)

	h := newTestHandler(bot, userUC, glucUC, loc)
	msg := testMessage(123, 456, "/start")
	msg.Entities = []tgbotapi.MessageEntity{{Type: "bot_command", Length: 6}}
	h.HandleStart(context.Background(), msg)

	require.Len(t, bot.sent, 1)
	sent := bot.lastMessage()
	kb := sent.ReplyMarkup.(tgbotapi.InlineKeyboardMarkup)
	unitsBtn := kb.InlineKeyboard[1][0]
	assert.Contains(t, unitsBtn.Text, "мг/дл")
}
