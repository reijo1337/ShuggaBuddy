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
	tgmocks "github.com/gmtantsevov/shuggabuddy/internal/delivery/telegram/mocks"
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
	foodUC telegram.FoodUseCase,
	insulinUC telegram.InsulinUseCase,
	activityUC telegram.ActivityUseCase,
	noteUC telegram.NoteUseCase,
	diaryUC telegram.DiaryUseCase,
	bolusUC telegram.BolusUseCase,
	advisorUC telegram.DoseAdvisorUseCase,
	loc *i18n.Localizer,
) *telegram.Handler {
	return telegram.NewHandler(bot, userUC, glucUC, foodUC, insulinUC, activityUC, noteUC, diaryUC, bolusUC, advisorUC, loc, zap.NewNop())
}

func testMessage(userID, chatID int64, text string) *tgbotapi.Message {
	return &tgbotapi.Message{
		From: &tgbotapi.User{ID: userID, FirstName: "Test"},
		Chat: &tgbotapi.Chat{ID: chatID},
		Text: text,
	}
}

func testCallback(userID, chatID int64, data string) *tgbotapi.CallbackQuery {
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
	ID:            1,
	Units:         domain.UnitsMmol,
	CarbsPerUnit:  12,
	TargetMinMmol: 3.9,
	TargetMaxMmol: 10.0,
	CreatedAt:     time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC),
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
	foodUC := tgmocks.NewMockFoodUseCase(ctrl)
	insulinUC := tgmocks.NewMockInsulinUseCase(ctrl)
	loc := newTestLocalizer(t)

	userUC.EXPECT().
		GetOrCreateUser(gomock.Any(), domain.ProviderTelegram, "123", "Test").
		Return(testUser, testAcc, true, nil)

	h := newTestHandler(bot, userUC, glucUC, foodUC, insulinUC, nil, nil, nil, nil, nil, loc)

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
	foodUC := tgmocks.NewMockFoodUseCase(ctrl)
	insulinUC := tgmocks.NewMockInsulinUseCase(ctrl)
	loc := newTestLocalizer(t)

	userUC.EXPECT().
		GetOrCreateUser(gomock.Any(), domain.ProviderTelegram, "123", "Test").
		Return(testUser, testAcc, false, nil)

	h := newTestHandler(bot, userUC, glucUC, foodUC, insulinUC, nil, nil, nil, nil, nil, loc)

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
	foodUC := tgmocks.NewMockFoodUseCase(ctrl)
	insulinUC := tgmocks.NewMockInsulinUseCase(ctrl)
	loc := newTestLocalizer(t)

	userUC.EXPECT().
		GetOrCreateUser(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, nil, false, errors.New("db error"))

	h := newTestHandler(bot, userUC, glucUC, foodUC, insulinUC, nil, nil, nil, nil, nil, loc)

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
	foodUC := tgmocks.NewMockFoodUseCase(ctrl)
	insulinUC := tgmocks.NewMockInsulinUseCase(ctrl)
	loc := newTestLocalizer(t)

	userUC.EXPECT().
		GetProfile(gomock.Any(), domain.ProviderTelegram, "123").
		Return(testUser, testAcc, nil)

	h := newTestHandler(bot, userUC, glucUC, foodUC, insulinUC, nil, nil, nil, nil, nil, loc)
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
	foodUC := tgmocks.NewMockFoodUseCase(ctrl)
	insulinUC := tgmocks.NewMockInsulinUseCase(ctrl)
	loc := newTestLocalizer(t)

	userUC.EXPECT().
		GetProfile(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, nil, errors.New("db error"))

	h := newTestHandler(bot, userUC, glucUC, foodUC, insulinUC, nil, nil, nil, nil, nil, loc)
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
	foodUC := tgmocks.NewMockFoodUseCase(ctrl)
	insulinUC := tgmocks.NewMockInsulinUseCase(ctrl)
	loc := newTestLocalizer(t)

	h := newTestHandler(bot, userUC, glucUC, foodUC, insulinUC, nil, nil, nil, nil, nil, loc)
	h.HandleCallback(context.Background(), testCallback(123, 456, "menu:units"))

	require.Len(t, bot.sent, 1)
	sent := bot.lastMessage()
	assert.Contains(t, sent.Text, "Выбери единицы измерения:")
	kb := sent.ReplyMarkup.(tgbotapi.InlineKeyboardMarkup)
	assert.Len(t, kb.InlineKeyboard, 2)
	assert.Equal(t, "units:mmol", *kb.InlineKeyboard[0][0].CallbackData)
	assert.Equal(t, "units:mgdl", *kb.InlineKeyboard[0][1].CallbackData)
	assert.Equal(t, "menu:profile", *kb.InlineKeyboard[1][0].CallbackData)
}

// --- Callback: units:mmol / units:mgdl ---

func TestHandleCallback_SetUnitsMmol(t *testing.T) {
	ctrl := gomock.NewController(t)
	bot := &spyBot{}
	userUC := tgmocks.NewMockUserUseCase(ctrl)
	glucUC := tgmocks.NewMockGlucoseUseCase(ctrl)
	foodUC := tgmocks.NewMockFoodUseCase(ctrl)
	insulinUC := tgmocks.NewMockInsulinUseCase(ctrl)
	loc := newTestLocalizer(t)

	userUC.EXPECT().
		GetProfile(gomock.Any(), domain.ProviderTelegram, "123").
		Return(testUser, testAcc, nil).Times(2)
	userUC.EXPECT().
		UpdateUnits(gomock.Any(), int64(1), domain.UnitsMmol).
		Return(nil)

	h := newTestHandler(bot, userUC, glucUC, foodUC, insulinUC, nil, nil, nil, nil, nil, loc)
	h.HandleCallback(context.Background(), testCallback(123, 456, "units:mmol"))

	require.Len(t, bot.sent, 1)
	sent := bot.lastMessage()
	assert.Contains(t, sent.Text, "Профиль")
	assert.Contains(t, sent.Text, "Test")
}

func TestHandleCallback_SetUnitsMgdl(t *testing.T) {
	ctrl := gomock.NewController(t)
	bot := &spyBot{}
	userUC := tgmocks.NewMockUserUseCase(ctrl)
	glucUC := tgmocks.NewMockGlucoseUseCase(ctrl)
	foodUC := tgmocks.NewMockFoodUseCase(ctrl)
	insulinUC := tgmocks.NewMockInsulinUseCase(ctrl)
	loc := newTestLocalizer(t)

	userUC.EXPECT().
		GetProfile(gomock.Any(), domain.ProviderTelegram, "123").
		Return(testUser, testAcc, nil).Times(2)
	userUC.EXPECT().
		UpdateUnits(gomock.Any(), int64(1), domain.UnitsMgdl).
		Return(nil)

	h := newTestHandler(bot, userUC, glucUC, foodUC, insulinUC, nil, nil, nil, nil, nil, loc)
	h.HandleCallback(context.Background(), testCallback(123, 456, "units:mgdl"))

	require.Len(t, bot.sent, 1)
	sent := bot.lastMessage()
	assert.Contains(t, sent.Text, "Профиль")
	assert.Contains(t, sent.Text, "Test")
}

func TestHandleCallback_SetUnits_UpdateError(t *testing.T) {
	ctrl := gomock.NewController(t)
	bot := &spyBot{}
	userUC := tgmocks.NewMockUserUseCase(ctrl)
	glucUC := tgmocks.NewMockGlucoseUseCase(ctrl)
	foodUC := tgmocks.NewMockFoodUseCase(ctrl)
	insulinUC := tgmocks.NewMockInsulinUseCase(ctrl)
	loc := newTestLocalizer(t)

	userUC.EXPECT().
		GetProfile(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(testUser, testAcc, nil)
	userUC.EXPECT().
		UpdateUnits(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(errors.New("db error"))

	h := newTestHandler(bot, userUC, glucUC, foodUC, insulinUC, nil, nil, nil, nil, nil, loc)
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
	foodUC := tgmocks.NewMockFoodUseCase(ctrl)
	insulinUC := tgmocks.NewMockInsulinUseCase(ctrl)
	loc := newTestLocalizer(t)

	h := newTestHandler(bot, userUC, glucUC, foodUC, insulinUC, nil, nil, nil, nil, nil, loc)
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
	foodUC := tgmocks.NewMockFoodUseCase(ctrl)
	insulinUC := tgmocks.NewMockInsulinUseCase(ctrl)
	loc := newTestLocalizer(t)

	userUC.EXPECT().
		GetProfile(gomock.Any(), domain.ProviderTelegram, "123").
		Return(testUser, testAcc, nil)
	glucUC.EXPECT().
		SaveReading(gomock.Any(), int64(1), 5.4, domain.UnitsMmol).
		Return(nil)

	h := newTestHandler(bot, userUC, glucUC, foodUC, insulinUC, nil, nil, nil, nil, nil, loc)
	sess := telegram.NewSession(telegram.SessionGlucose, telegram.StepGlucoseValue)
	h.HandleSessionInput(context.Background(), testMessage(123, 456, "5.4"), sess)

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
	foodUC := tgmocks.NewMockFoodUseCase(ctrl)
	insulinUC := tgmocks.NewMockInsulinUseCase(ctrl)
	loc := newTestLocalizer(t)

	h := newTestHandler(bot, userUC, glucUC, foodUC, insulinUC, nil, nil, nil, nil, nil, loc)
	sess := telegram.NewSession(telegram.SessionGlucose, telegram.StepGlucoseValue)
	h.HandleSessionInput(context.Background(), testMessage(123, 456, "abc"), sess)

	require.Len(t, bot.sent, 1)
	assert.Contains(t, bot.lastMessage().Text, "Некорректное значение")
}

func TestHandleGlucoseInput_OutOfRange(t *testing.T) {
	ctrl := gomock.NewController(t)
	bot := &spyBot{}
	userUC := tgmocks.NewMockUserUseCase(ctrl)
	glucUC := tgmocks.NewMockGlucoseUseCase(ctrl)
	foodUC := tgmocks.NewMockFoodUseCase(ctrl)
	insulinUC := tgmocks.NewMockInsulinUseCase(ctrl)
	loc := newTestLocalizer(t)

	userUC.EXPECT().
		GetProfile(gomock.Any(), domain.ProviderTelegram, "123").
		Return(testUser, testAcc, nil)
	glucUC.EXPECT().
		SaveReading(gomock.Any(), int64(1), 50.0, domain.UnitsMmol).
		Return(errors.New("out of range"))

	h := newTestHandler(bot, userUC, glucUC, foodUC, insulinUC, nil, nil, nil, nil, nil, loc)
	sess := telegram.NewSession(telegram.SessionGlucose, telegram.StepGlucoseValue)
	h.HandleSessionInput(context.Background(), testMessage(123, 456, "50.0"), sess)

	require.Len(t, bot.sent, 1)
	assert.Contains(t, bot.lastMessage().Text, "вне допустимого диапазона")
}

func TestHandleGlucoseInput_OutOfRange_Mgdl(t *testing.T) {
	ctrl := gomock.NewController(t)
	bot := &spyBot{}
	userUC := tgmocks.NewMockUserUseCase(ctrl)
	glucUC := tgmocks.NewMockGlucoseUseCase(ctrl)
	foodUC := tgmocks.NewMockFoodUseCase(ctrl)
	insulinUC := tgmocks.NewMockInsulinUseCase(ctrl)
	loc := newTestLocalizer(t)

	mgdlUser := &domain.User{ID: 1, Units: domain.UnitsMgdl}

	userUC.EXPECT().
		GetProfile(gomock.Any(), domain.ProviderTelegram, "123").
		Return(mgdlUser, testAcc, nil)
	glucUC.EXPECT().
		SaveReading(gomock.Any(), int64(1), 700.0, domain.UnitsMgdl).
		Return(errors.New("out of range"))

	h := newTestHandler(bot, userUC, glucUC, foodUC, insulinUC, nil, nil, nil, nil, nil, loc)
	sess := telegram.NewSession(telegram.SessionGlucose, telegram.StepGlucoseValue)
	h.HandleSessionInput(context.Background(), testMessage(123, 456, "700"), sess)

	require.Len(t, bot.sent, 1)
	assert.Contains(t, bot.lastMessage().Text, "мг/дл")
}

// --- Callback: menu:back ---

func TestHandleCallback_Back(t *testing.T) {
	bot := &spyBot{}
	ctrl := gomock.NewController(t)
	userUC := tgmocks.NewMockUserUseCase(ctrl)
	glucUC := tgmocks.NewMockGlucoseUseCase(ctrl)
	foodUC := tgmocks.NewMockFoodUseCase(ctrl)
	insulinUC := tgmocks.NewMockInsulinUseCase(ctrl)
	loc := newTestLocalizer(t)

	h := newTestHandler(bot, userUC, glucUC, foodUC, insulinUC, nil, nil, nil, nil, nil, loc)
	h.HandleCallback(context.Background(), testCallback(123, 456, "menu:back"))

	require.Len(t, bot.sent, 1)
	sent := bot.lastMessage()
	assert.Contains(t, sent.Text, "Выбери действие:")
	kb := sent.ReplyMarkup.(tgbotapi.InlineKeyboardMarkup)
	assert.Len(t, kb.InlineKeyboard, 5)
}

// --- Waiting glucose state ---

func TestHandleCallback_Back_ClearsSession(t *testing.T) {
	ctrl := gomock.NewController(t)
	bot := &spyBot{}
	userUC := tgmocks.NewMockUserUseCase(ctrl)
	glucUC := tgmocks.NewMockGlucoseUseCase(ctrl)
	foodUC := tgmocks.NewMockFoodUseCase(ctrl)
	insulinUC := tgmocks.NewMockInsulinUseCase(ctrl)
	loc := newTestLocalizer(t)

	h := newTestHandler(bot, userUC, glucUC, foodUC, insulinUC, nil, nil, nil, nil, nil, loc)

	// Активируем ожидание ввода глюкозы.
	h.HandleCallback(context.Background(), testCallback(123, 456, "menu:glucose"))
	require.Len(t, bot.sent, 1)

	// Нажимаем "В меню" — состояние ожидания должно очиститься.
	h.HandleCallback(context.Background(), testCallback(123, 456, "menu:back"))
	require.Len(t, bot.sent, 2)
	assert.Contains(t, bot.sent[1].(tgbotapi.MessageConfig).Text, "Выбери действие:")
}

// --- Callback: menu:new_entry ---

func TestHandleCallback_NewEntry(t *testing.T) {
	bot := &spyBot{}
	ctrl := gomock.NewController(t)
	userUC := tgmocks.NewMockUserUseCase(ctrl)
	loc := newTestLocalizer(t)

	h := newTestHandler(bot, userUC, nil, nil, nil, nil, nil, nil, nil, nil, loc)
	h.HandleCallback(context.Background(), testCallback(123, 456, "menu:new_entry"))

	require.Len(t, bot.sent, 1)
	kb, ok := bot.lastMessage().ReplyMarkup.(tgbotapi.InlineKeyboardMarkup)
	require.True(t, ok)

	var callbacks []string
	for _, row := range kb.InlineKeyboard {
		for _, btn := range row {
			if btn.CallbackData != nil {
				callbacks = append(callbacks, *btn.CallbackData)
			}
		}
	}
	assert.Contains(t, callbacks, "menu:glucose")
	assert.Contains(t, callbacks, "menu:food")
	assert.Contains(t, callbacks, "menu:insulin")
	assert.Contains(t, callbacks, "menu:activity")
	assert.Contains(t, callbacks, "menu:note")
	assert.Contains(t, callbacks, "menu:back")
}

// --- Food flow ---

func TestHandleCallback_FoodStart(t *testing.T) {
	ctrl := gomock.NewController(t)
	bot := &spyBot{}
	userUC := tgmocks.NewMockUserUseCase(ctrl)
	glucUC := tgmocks.NewMockGlucoseUseCase(ctrl)
	foodUC := tgmocks.NewMockFoodUseCase(ctrl)
	insulinUC := tgmocks.NewMockInsulinUseCase(ctrl)
	loc := newTestLocalizer(t)

	userUC.EXPECT().
		GetProfile(gomock.Any(), domain.ProviderTelegram, "123").
		Return(testUser, testAcc, nil)

	h := newTestHandler(bot, userUC, glucUC, foodUC, insulinUC, nil, nil, nil, nil, nil, loc)
	h.HandleCallback(context.Background(), testCallback(123, 456, "menu:food"))

	require.Len(t, bot.sent, 1)
	sent := bot.lastMessage()
	assert.Contains(t, sent.Text, "Сколько углеводов?")
	kb := sent.ReplyMarkup.(tgbotapi.InlineKeyboardMarkup)
	// First row: unit toggle [г ✓] [ХЕ]
	assert.Equal(t, "food:unit:g", *kb.InlineKeyboard[0][0].CallbackData)
	assert.Equal(t, "food:unit:xe", *kb.InlineKeyboard[0][1].CallbackData)
}

func TestHandleFoodCarbsInput_Valid_Grams(t *testing.T) {
	ctrl := gomock.NewController(t)
	bot := &spyBot{}
	userUC := tgmocks.NewMockUserUseCase(ctrl)
	glucUC := tgmocks.NewMockGlucoseUseCase(ctrl)
	foodUC := tgmocks.NewMockFoodUseCase(ctrl)
	insulinUC := tgmocks.NewMockInsulinUseCase(ctrl)
	loc := newTestLocalizer(t)

	h := newTestHandler(bot, userUC, glucUC, foodUC, insulinUC, nil, nil, nil, nil, nil, loc)
	sess := telegram.NewSession(telegram.SessionFood, telegram.StepFoodCarbs)
	sess.Data["carbs_unit"] = "g"

	h.HandleSessionInput(context.Background(), testMessage(123, 456, "60"), sess)

	require.Len(t, bot.sent, 1)
	assert.Contains(t, bot.lastMessage().Text, "заметку")
}

func TestHandleFoodCarbsInput_Valid_XE(t *testing.T) {
	ctrl := gomock.NewController(t)
	bot := &spyBot{}
	userUC := tgmocks.NewMockUserUseCase(ctrl)
	glucUC := tgmocks.NewMockGlucoseUseCase(ctrl)
	foodUC := tgmocks.NewMockFoodUseCase(ctrl)
	insulinUC := tgmocks.NewMockInsulinUseCase(ctrl)
	loc := newTestLocalizer(t)

	userUC.EXPECT().
		GetProfile(gomock.Any(), domain.ProviderTelegram, "123").
		Return(testUser, testAcc, nil)

	h := newTestHandler(bot, userUC, glucUC, foodUC, insulinUC, nil, nil, nil, nil, nil, loc)
	sess := telegram.NewSession(telegram.SessionFood, telegram.StepFoodCarbs)
	sess.Data["carbs_unit"] = "xe"

	// 5 XE * 12 g/XE = 60g — within range
	h.HandleSessionInput(context.Background(), testMessage(123, 456, "5"), sess)

	require.Len(t, bot.sent, 1)
	assert.Contains(t, bot.lastMessage().Text, "заметку")
}

func TestHandleFoodCarbsInput_InvalidText(t *testing.T) {
	ctrl := gomock.NewController(t)
	bot := &spyBot{}
	userUC := tgmocks.NewMockUserUseCase(ctrl)
	glucUC := tgmocks.NewMockGlucoseUseCase(ctrl)
	foodUC := tgmocks.NewMockFoodUseCase(ctrl)
	insulinUC := tgmocks.NewMockInsulinUseCase(ctrl)
	loc := newTestLocalizer(t)

	h := newTestHandler(bot, userUC, glucUC, foodUC, insulinUC, nil, nil, nil, nil, nil, loc)
	sess := telegram.NewSession(telegram.SessionFood, telegram.StepFoodCarbs)
	sess.Data["carbs_unit"] = "g"

	h.HandleSessionInput(context.Background(), testMessage(123, 456, "abc"), sess)

	require.Len(t, bot.sent, 1)
	assert.Contains(t, bot.lastMessage().Text, "Некорректное значение")
}

func TestHandleFoodTimeInput_ValidFormats(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"HH:MM", "13:45"},
		{"DD.MM HH:MM", "02.03 13:45"},
		{"DD.MM.YYYY HH:MM", "02.03.2025 13:45"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			bot := &spyBot{}
			userUC := tgmocks.NewMockUserUseCase(ctrl)
			glucUC := tgmocks.NewMockGlucoseUseCase(ctrl)
			foodUC := tgmocks.NewMockFoodUseCase(ctrl)
			insulinUC := tgmocks.NewMockInsulinUseCase(ctrl)
			loc := newTestLocalizer(t)

			userUC.EXPECT().
				GetProfile(gomock.Any(), gomock.Any(), gomock.Any()).
				Return(testUser, testAcc, nil)
			foodUC.EXPECT().
				SaveEntry(gomock.Any(), int64(1), float64(60), "", gomock.Any()).
				Return(nil)

			h := newTestHandler(bot, userUC, glucUC, foodUC, insulinUC, nil, nil, nil, nil, nil, loc)
			sess := telegram.NewSession(telegram.SessionFood, telegram.StepFoodTime)
			sess.Data["carbs_grams"] = float64(60)
			sess.Data["note"] = ""

			h.HandleSessionInput(context.Background(), testMessage(123, 456, tt.input), sess)

			require.Len(t, bot.sent, 1)
			assert.Contains(t, bot.lastMessage().Text, "Записано")
		})
	}
}

func TestHandleFoodTimeInput_InvalidFormat(t *testing.T) {
	ctrl := gomock.NewController(t)
	bot := &spyBot{}
	userUC := tgmocks.NewMockUserUseCase(ctrl)
	glucUC := tgmocks.NewMockGlucoseUseCase(ctrl)
	foodUC := tgmocks.NewMockFoodUseCase(ctrl)
	insulinUC := tgmocks.NewMockInsulinUseCase(ctrl)
	loc := newTestLocalizer(t)

	h := newTestHandler(bot, userUC, glucUC, foodUC, insulinUC, nil, nil, nil, nil, nil, loc)
	sess := telegram.NewSession(telegram.SessionFood, telegram.StepFoodTime)
	sess.Data["carbs_grams"] = float64(60)
	sess.Data["note"] = ""

	h.HandleSessionInput(context.Background(), testMessage(123, 456, "not-a-time"), sess)

	require.Len(t, bot.sent, 1)
	assert.Contains(t, bot.lastMessage().Text, "Не удалось распознать")
}

func TestHandleCallback_CarbsUnit_SetPreset(t *testing.T) {
	ctrl := gomock.NewController(t)
	bot := &spyBot{}
	userUC := tgmocks.NewMockUserUseCase(ctrl)
	glucUC := tgmocks.NewMockGlucoseUseCase(ctrl)
	foodUC := tgmocks.NewMockFoodUseCase(ctrl)
	insulinUC := tgmocks.NewMockInsulinUseCase(ctrl)
	loc := newTestLocalizer(t)

	userUC.EXPECT().
		GetProfile(gomock.Any(), domain.ProviderTelegram, "123").
		Return(testUser, testAcc, nil).Times(2)
	userUC.EXPECT().
		UpdateCarbsPerUnit(gomock.Any(), int64(1), 10.0).
		Return(nil)

	h := newTestHandler(bot, userUC, glucUC, foodUC, insulinUC, nil, nil, nil, nil, nil, loc)
	h.HandleCallback(context.Background(), testCallback(123, 456, "carbs_unit:10"))

	require.Len(t, bot.sent, 1)
	sent := bot.lastMessage()
	assert.Contains(t, sent.Text, "Профиль")
	assert.Contains(t, sent.Text, "Test")
}

// --- Analytics ---

func TestHandleCallback_Analytics(t *testing.T) {
	ctrl := gomock.NewController(t)
	bot := &spyBot{}
	userUC := tgmocks.NewMockUserUseCase(ctrl)
	activityUC := tgmocks.NewMockActivityUseCase(ctrl)
	loc := newTestLocalizer(t)

	now := time.Date(2026, 3, 16, 10, 0, 0, 0, time.UTC)
	before := 6.0
	after := 4.5
	delta := -1.5
	tbefore := now.Add(-20 * time.Minute)
	tafter := now.Add(90 * time.Minute)

	analyses := []domain.ActivityAnalysis{
		{
			Entry: domain.ActivityEntry{
				ID: 1, ActivityType: domain.ActivityRunning, DurationMin: 30,
				Intensity: domain.IntensityMedium, RecordedAt: now,
			},
			GlucBefore: &before,
			GlucAfter:  &after,
			Delta:      &delta,
			TimeBefore: &tbefore,
			TimeAfter:  &tafter,
		},
	}

	userUC.EXPECT().
		GetProfile(gomock.Any(), domain.ProviderTelegram, "123").
		Return(testUser, testAcc, nil)
	activityUC.EXPECT().
		AnalyzeLastActivities(gomock.Any(), int64(1), 5).
		Return(analyses, nil)

	h := newTestHandler(bot, userUC, nil, nil, nil, activityUC, nil, nil, nil, nil, loc)
	h.HandleCallback(context.Background(), testCallback(123, 456, "menu:analytics"))

	require.Len(t, bot.sent, 1)
	text := bot.lastMessage().Text
	assert.Contains(t, text, "Анализ")
	assert.Contains(t, text, "6.0")
	assert.Contains(t, text, "4.5")
	assert.Contains(t, text, "-1.5")
}

func TestHandleCallback_Analytics_Empty(t *testing.T) {
	ctrl := gomock.NewController(t)
	bot := &spyBot{}
	userUC := tgmocks.NewMockUserUseCase(ctrl)
	activityUC := tgmocks.NewMockActivityUseCase(ctrl)
	loc := newTestLocalizer(t)

	userUC.EXPECT().
		GetProfile(gomock.Any(), domain.ProviderTelegram, "123").
		Return(testUser, testAcc, nil)
	activityUC.EXPECT().
		AnalyzeLastActivities(gomock.Any(), int64(1), 5).
		Return(nil, nil)

	h := newTestHandler(bot, userUC, nil, nil, nil, activityUC, nil, nil, nil, nil, loc)
	h.HandleCallback(context.Background(), testCallback(123, 456, "menu:analytics"))

	require.Len(t, bot.sent, 1)
	assert.Contains(t, bot.lastMessage().Text, "Нет записей")
}
