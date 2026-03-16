package telegram_test

import (
	"context"
	"errors"
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/gmtantsevov/shuggabuddy/internal/delivery/telegram"
	tgmocks "github.com/gmtantsevov/shuggabuddy/internal/delivery/telegram/mocks"
	"github.com/gmtantsevov/shuggabuddy/internal/domain"
)

// --- Callback: menu:insulin ---

func TestHandleCallback_InsulinStart(t *testing.T) {
	ctrl := gomock.NewController(t)
	bot := &spyBot{}
	userUC := tgmocks.NewMockUserUseCase(ctrl)
	glucUC := tgmocks.NewMockGlucoseUseCase(ctrl)
	foodUC := tgmocks.NewMockFoodUseCase(ctrl)
	insulinUC := tgmocks.NewMockInsulinUseCase(ctrl)
	loc := newTestLocalizer(t)

	h := newTestHandler(bot, userUC, glucUC, foodUC, insulinUC, nil, loc)
	h.HandleCallback(context.Background(), testCallback(123, 456, "menu:insulin"))

	require.Len(t, bot.sent, 1)
	sent := bot.lastMessage()
	assert.Contains(t, sent.Text, "Выбери тип инсулина")
	kb := sent.ReplyMarkup.(tgbotapi.InlineKeyboardMarkup)
	assert.Equal(t, "insulin:type:bolus", *kb.InlineKeyboard[0][0].CallbackData)
	assert.Equal(t, "insulin:type:basal", *kb.InlineKeyboard[0][1].CallbackData)
}

// --- Callback: insulin:type:bolus → prompt for dose ---

func TestHandleCallback_InsulinTypeBolus(t *testing.T) {
	ctrl := gomock.NewController(t)
	bot := &spyBot{}
	userUC := tgmocks.NewMockUserUseCase(ctrl)
	glucUC := tgmocks.NewMockGlucoseUseCase(ctrl)
	foodUC := tgmocks.NewMockFoodUseCase(ctrl)
	insulinUC := tgmocks.NewMockInsulinUseCase(ctrl)
	loc := newTestLocalizer(t)

	h := newTestHandler(bot, userUC, glucUC, foodUC, insulinUC, nil, loc)
	h.HandleCallback(context.Background(), testCallback(123, 456, "menu:insulin"))
	bot.sent = nil

	h.HandleCallback(context.Background(), testCallback(123, 456, "insulin:type:bolus"))
	require.Len(t, bot.sent, 1)
	assert.Contains(t, bot.lastMessage().Text, "Введи дозу")
}

// --- Dose input: valid, no anomaly → drug prompt ---

func TestHandleInsulinDoseInput_Valid_NoAnomaly(t *testing.T) {
	ctrl := gomock.NewController(t)
	bot := &spyBot{}
	userUC := tgmocks.NewMockUserUseCase(ctrl)
	glucUC := tgmocks.NewMockGlucoseUseCase(ctrl)
	foodUC := tgmocks.NewMockFoodUseCase(ctrl)
	insulinUC := tgmocks.NewMockInsulinUseCase(ctrl)
	loc := newTestLocalizer(t)

	h := newTestHandler(bot, userUC, glucUC, foodUC, insulinUC, nil, loc)
	sess := telegram.NewSession(telegram.SessionInsulin, telegram.StepInsulinDose)
	sess.Data["type"] = string(domain.InsulinTypeBolus)

	h.HandleSessionInput(context.Background(), testMessage(123, 456, "8"), sess)

	require.Len(t, bot.sent, 1)
	assert.Contains(t, bot.lastMessage().Text, "препарат")
	kb := bot.lastMessage().ReplyMarkup.(tgbotapi.InlineKeyboardMarkup)
	found := false
	for _, row := range kb.InlineKeyboard {
		for _, btn := range row {
			if btn.CallbackData != nil && *btn.CallbackData == "insulin:skip_drug" {
				found = true
			}
		}
	}
	assert.True(t, found, "expected insulin:skip_drug button")
}

// --- Dose input: invalid text ---

func TestHandleInsulinDoseInput_InvalidText(t *testing.T) {
	ctrl := gomock.NewController(t)
	bot := &spyBot{}
	userUC := tgmocks.NewMockUserUseCase(ctrl)
	glucUC := tgmocks.NewMockGlucoseUseCase(ctrl)
	foodUC := tgmocks.NewMockFoodUseCase(ctrl)
	insulinUC := tgmocks.NewMockInsulinUseCase(ctrl)
	loc := newTestLocalizer(t)

	h := newTestHandler(bot, userUC, glucUC, foodUC, insulinUC, nil, loc)
	sess := telegram.NewSession(telegram.SessionInsulin, telegram.StepInsulinDose)
	sess.Data["type"] = string(domain.InsulinTypeBolus)

	h.HandleSessionInput(context.Background(), testMessage(123, 456, "abc"), sess)

	require.Len(t, bot.sent, 1)
	assert.Contains(t, bot.lastMessage().Text, "Некорректное значение")
}

// --- Dose input: zero ---

func TestHandleInsulinDoseInput_Zero(t *testing.T) {
	ctrl := gomock.NewController(t)
	bot := &spyBot{}
	userUC := tgmocks.NewMockUserUseCase(ctrl)
	glucUC := tgmocks.NewMockGlucoseUseCase(ctrl)
	foodUC := tgmocks.NewMockFoodUseCase(ctrl)
	insulinUC := tgmocks.NewMockInsulinUseCase(ctrl)
	loc := newTestLocalizer(t)

	h := newTestHandler(bot, userUC, glucUC, foodUC, insulinUC, nil, loc)
	sess := telegram.NewSession(telegram.SessionInsulin, telegram.StepInsulinDose)
	sess.Data["type"] = string(domain.InsulinTypeBolus)

	h.HandleSessionInput(context.Background(), testMessage(123, 456, "0"), sess)

	require.Len(t, bot.sent, 1)
	assert.Contains(t, bot.lastMessage().Text, "Некорректное значение")
}

// --- Dose input: anomalously large → warning with confirm/cancel ---

func TestHandleInsulinDoseInput_Anomalous_ShowsWarning(t *testing.T) {
	ctrl := gomock.NewController(t)
	bot := &spyBot{}
	userUC := tgmocks.NewMockUserUseCase(ctrl)
	glucUC := tgmocks.NewMockGlucoseUseCase(ctrl)
	foodUC := tgmocks.NewMockFoodUseCase(ctrl)
	insulinUC := tgmocks.NewMockInsulinUseCase(ctrl)
	loc := newTestLocalizer(t)

	h := newTestHandler(bot, userUC, glucUC, foodUC, insulinUC, nil, loc)
	sess := telegram.NewSession(telegram.SessionInsulin, telegram.StepInsulinDose)
	sess.Data["type"] = string(domain.InsulinTypeBolus)

	// 60 ед. болюса > порог 50
	h.HandleSessionInput(context.Background(), testMessage(123, 456, "60"), sess)

	require.Len(t, bot.sent, 1)
	text := bot.lastMessage().Text
	assert.Contains(t, text, "60")
	assert.Contains(t, text, "уверены")
	kb := bot.lastMessage().ReplyMarkup.(tgbotapi.InlineKeyboardMarkup)
	var hasConfirm, hasCancel bool
	for _, row := range kb.InlineKeyboard {
		for _, btn := range row {
			if btn.CallbackData != nil {
				switch *btn.CallbackData {
				case "insulin:confirm":
					hasConfirm = true
				case "insulin:cancel":
					hasCancel = true
				}
			}
		}
	}
	assert.True(t, hasConfirm, "expected insulin:confirm button")
	assert.True(t, hasCancel, "expected insulin:cancel button")
}

// --- Confirm anomalous dose → drug prompt ---

func TestHandleCallback_InsulinConfirm(t *testing.T) {
	ctrl := gomock.NewController(t)
	bot := &spyBot{}
	userUC := tgmocks.NewMockUserUseCase(ctrl)
	glucUC := tgmocks.NewMockGlucoseUseCase(ctrl)
	foodUC := tgmocks.NewMockFoodUseCase(ctrl)
	insulinUC := tgmocks.NewMockInsulinUseCase(ctrl)
	loc := newTestLocalizer(t)

	h := newTestHandler(bot, userUC, glucUC, foodUC, insulinUC, nil, loc)
	sess := telegram.NewSession(telegram.SessionInsulin, telegram.StepInsulinConfirm)
	sess.Data["type"] = string(domain.InsulinTypeBolus)
	sess.Data["dose"] = float64(60)
	h.SetSession(456, sess)

	h.HandleCallback(context.Background(), testCallback(123, 456, "insulin:confirm"))

	require.Len(t, bot.sent, 1)
	assert.Contains(t, bot.lastMessage().Text, "препарат")
}

// --- Skip drug → save ---

func TestHandleCallback_InsulinSkipDrug(t *testing.T) {
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
	insulinUC.EXPECT().
		SaveDose(gomock.Any(), int64(1), float64(8), domain.InsulinTypeBolus, "").
		Return(nil)

	h := newTestHandler(bot, userUC, glucUC, foodUC, insulinUC, nil, loc)
	sess := telegram.NewSession(telegram.SessionInsulin, telegram.StepInsulinDrug)
	sess.Data["type"] = string(domain.InsulinTypeBolus)
	sess.Data["dose"] = float64(8)
	h.SetSession(456, sess)

	h.HandleCallback(context.Background(), testCallback(123, 456, "insulin:skip_drug"))

	require.Len(t, bot.sent, 1)
	text := bot.lastMessage().Text
	assert.Contains(t, text, "Инсулин записан")
	assert.Contains(t, text, "8")
}

// --- Drug name input → save with drug ---

func TestHandleInsulinDrugInput_WithDrug(t *testing.T) {
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
	insulinUC.EXPECT().
		SaveDose(gomock.Any(), int64(1), float64(20), domain.InsulinTypeBasal, "Лантус").
		Return(nil)

	h := newTestHandler(bot, userUC, glucUC, foodUC, insulinUC, nil, loc)
	sess := telegram.NewSession(telegram.SessionInsulin, telegram.StepInsulinDrug)
	sess.Data["type"] = string(domain.InsulinTypeBasal)
	sess.Data["dose"] = float64(20)

	h.HandleSessionInput(context.Background(), testMessage(123, 456, "Лантус"), sess)

	require.Len(t, bot.sent, 1)
	text := bot.lastMessage().Text
	assert.Contains(t, text, "Инсулин записан")
	assert.Contains(t, text, "Лантус")
}

// --- SaveDose error ---

func TestHandleInsulinDrugInput_SaveError(t *testing.T) {
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
	insulinUC.EXPECT().
		SaveDose(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(errors.New("db error"))

	h := newTestHandler(bot, userUC, glucUC, foodUC, insulinUC, nil, loc)
	sess := telegram.NewSession(telegram.SessionInsulin, telegram.StepInsulinDrug)
	sess.Data["type"] = string(domain.InsulinTypeBolus)
	sess.Data["dose"] = float64(8)

	h.HandleSessionInput(context.Background(), testMessage(123, 456, ""), sess)

	require.Len(t, bot.sent, 1)
	assert.Contains(t, bot.lastMessage().Text, "внутренняя ошибка")
}

// --- Cancel anomalous dose → back to menu ---

func TestHandleCallback_InsulinCancel(t *testing.T) {
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

	h := newTestHandler(bot, userUC, glucUC, foodUC, insulinUC, nil, loc)
	sess := telegram.NewSession(telegram.SessionInsulin, telegram.StepInsulinConfirm)
	sess.Data["type"] = string(domain.InsulinTypeBolus)
	sess.Data["dose"] = float64(60)
	h.SetSession(456, sess)

	h.HandleCallback(context.Background(), testCallback(123, 456, "insulin:cancel"))

	require.Len(t, bot.sent, 1)
	assert.Contains(t, bot.lastMessage().Text, "Выбери действие")
}
