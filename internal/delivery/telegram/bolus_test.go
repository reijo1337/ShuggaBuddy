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

	"github.com/gmtantsevov/shuggabuddy/internal/delivery/telegram"
	tgmocks "github.com/gmtantsevov/shuggabuddy/internal/delivery/telegram/mocks"
	"github.com/gmtantsevov/shuggabuddy/internal/domain"
)

// TestBolusStart_NoDrugSet verifies that when the user has no bolus drug configured,
// the handler returns an error message prompting to configure it.
func TestBolusStart_NoDrugSet(t *testing.T) {
	ctrl := gomock.NewController(t)
	bot := &spyBot{}
	userUC := tgmocks.NewMockUserUseCase(ctrl)
	glucUC := tgmocks.NewMockGlucoseUseCase(ctrl)
	bolusUC := tgmocks.NewMockBolusUseCase(ctrl)
	loc := newTestLocalizer(t)

	user := &domain.User{ID: 1, BolusDrug: ""}
	acc := &domain.ExternalAccount{UserID: 1, Provider: domain.ProviderTelegram, ExternalID: "123"}

	userUC.EXPECT().GetProfile(gomock.Any(), domain.ProviderTelegram, "123").Return(user, acc, nil)

	h := newTestHandler(bot, userUC, glucUC, nil, nil, nil, nil, nil, bolusUC, nil, loc)
	h.HandleCallback(context.Background(), testCallback(123, 456, "bolus:start"))

	require.Len(t, bot.sent, 1)
	msg := bot.lastMessage()
	assert.Contains(t, msg.Text, "болюсный инсулин")

	kb := msg.ReplyMarkup.(tgbotapi.InlineKeyboardMarkup)
	found := false
	for _, row := range kb.InlineKeyboard {
		for _, btn := range row {
			if btn.CallbackData != nil && *btn.CallbackData == "profile:bolus_drug" {
				found = true
			}
		}
	}
	assert.True(t, found, "expected profile:bolus_drug button in keyboard")
}

// TestBolusStart_FreshGlucose verifies that when a fresh glucose reading exists (within 30 min),
// the handler shows the glucose value and confirmation buttons.
func TestBolusStart_FreshGlucose(t *testing.T) {
	ctrl := gomock.NewController(t)
	bot := &spyBot{}
	userUC := tgmocks.NewMockUserUseCase(ctrl)
	glucUC := tgmocks.NewMockGlucoseUseCase(ctrl)
	bolusUC := tgmocks.NewMockBolusUseCase(ctrl)
	loc := newTestLocalizer(t)

	user := &domain.User{
		ID:            1,
		BolusDrug:     "novorapid",
		Units:         domain.UnitsMmol,
		TargetMinMmol: 3.9,
		TargetMaxMmol: 10.0,
	}
	acc := &domain.ExternalAccount{UserID: 1, Provider: domain.ProviderTelegram, ExternalID: "123"}

	readings := []domain.GlucoseReading{
		{ID: 1, UserID: 1, ValueMmol: 7.5, RecordedAt: time.Now().Add(-10 * time.Minute)},
	}

	userUC.EXPECT().GetProfile(gomock.Any(), domain.ProviderTelegram, "123").Return(user, acc, nil)
	glucUC.EXPECT().GetLastReadings(gomock.Any(), int64(1), 1).Return(readings, nil)

	h := newTestHandler(bot, userUC, glucUC, nil, nil, nil, nil, nil, bolusUC, nil, loc)
	h.HandleCallback(context.Background(), testCallback(123, 456, "bolus:start"))

	require.Len(t, bot.sent, 1)
	msg := bot.lastMessage()
	assert.Contains(t, msg.Text, "7.5")

	kb := msg.ReplyMarkup.(tgbotapi.InlineKeyboardMarkup)
	var hasConfirm, hasManual bool
	for _, row := range kb.InlineKeyboard {
		for _, btn := range row {
			if btn.CallbackData != nil {
				switch *btn.CallbackData {
				case "bolus:glucose:confirm":
					hasConfirm = true
				case "bolus:glucose:manual":
					hasManual = true
				}
			}
		}
	}
	assert.True(t, hasConfirm, "expected bolus:glucose:confirm button")
	assert.True(t, hasManual, "expected bolus:glucose:manual button")
}

// TestBolusStart_NoFreshGlucose verifies that when no fresh glucose reading exists,
// the handler prompts for manual glucose entry.
func TestBolusStart_NoFreshGlucose(t *testing.T) {
	ctrl := gomock.NewController(t)
	bot := &spyBot{}
	userUC := tgmocks.NewMockUserUseCase(ctrl)
	glucUC := tgmocks.NewMockGlucoseUseCase(ctrl)
	bolusUC := tgmocks.NewMockBolusUseCase(ctrl)
	loc := newTestLocalizer(t)

	user := &domain.User{
		ID:            1,
		BolusDrug:     "novorapid",
		Units:         domain.UnitsMmol,
		TargetMinMmol: 3.9,
		TargetMaxMmol: 10.0,
	}
	acc := &domain.ExternalAccount{UserID: 1, Provider: domain.ProviderTelegram, ExternalID: "123"}

	userUC.EXPECT().GetProfile(gomock.Any(), domain.ProviderTelegram, "123").Return(user, acc, nil)
	glucUC.EXPECT().GetLastReadings(gomock.Any(), int64(1), 1).Return([]domain.GlucoseReading{}, nil)

	h := newTestHandler(bot, userUC, glucUC, nil, nil, nil, nil, nil, bolusUC, nil, loc)
	h.HandleCallback(context.Background(), testCallback(123, 456, "bolus:start"))

	require.Len(t, bot.sent, 1)
	msg := bot.lastMessage()
	assert.Contains(t, msg.Text, "Введи текущий уровень сахара")
}

// TestBolusCalculation_InsufficientData verifies that when bolusUC.Calculate returns an error,
// the handler shows the "insufficient data" message.
func TestBolusCalculation_InsufficientData(t *testing.T) {
	ctrl := gomock.NewController(t)
	bot := &spyBot{}
	userUC := tgmocks.NewMockUserUseCase(ctrl)
	glucUC := tgmocks.NewMockGlucoseUseCase(ctrl)
	bolusUC := tgmocks.NewMockBolusUseCase(ctrl)
	loc := newTestLocalizer(t)

	bolusUC.EXPECT().
		Calculate(gomock.Any(), int64(1), float64(7.5), gomock.Any(), gomock.Any()).
		Return(nil, errors.New("insufficient data"))

	h := newTestHandler(bot, userUC, glucUC, nil, nil, nil, nil, nil, bolusUC, nil, loc)

	sess := telegram.NewSession(telegram.SessionBolus, telegram.StepBolusCarbs)
	sess.Data["user_id"] = int64(1)
	sess.Data["glucose"] = float64(7.5)
	sess.Data["bolus_drug"] = "novorapid"
	h.SetSession(456, sess)

	h.HandleSessionInput(context.Background(), testMessage(123, 456, "60"), sess)

	require.Len(t, bot.sent, 1)
	msg := bot.lastMessage()
	assert.Contains(t, msg.Text, "Недостаточно данных")
}

// TestBolusSave_Success verifies that when a recommendation is stored in the session,
// the save callback stores the dose and confirms to the user.
func TestBolusSave_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	bot := &spyBot{}
	userUC := tgmocks.NewMockUserUseCase(ctrl)
	glucUC := tgmocks.NewMockGlucoseUseCase(ctrl)
	insulinUC := tgmocks.NewMockInsulinUseCase(ctrl)
	bolusUC := tgmocks.NewMockBolusUseCase(ctrl)
	loc := newTestLocalizer(t)

	rec := &domain.BolusRecommendation{
		TotalDose:      5.0,
		FoodDose:       4.0,
		CorrectionDose: 1.0,
		IOB:            0.0,
		ICR:            12.0,
		ISF:            2.5,
	}

	// "novorapid" maps to Name: "NovoRapid"
	insulinUC.EXPECT().
		SaveDose(gomock.Any(), int64(1), float64(5.0), domain.InsulinTypeBolus, "NovoRapid", "bolus_calculator").
		Return(nil)

	h := newTestHandler(bot, userUC, glucUC, nil, insulinUC, nil, nil, nil, bolusUC, nil, loc)

	sess := telegram.NewSession(telegram.SessionBolus, telegram.StepBolusCarbs)
	sess.Data["user_id"] = int64(1)
	sess.Data["glucose"] = float64(7.5)
	sess.Data["bolus_drug"] = "novorapid"
	sess.Data["recommendation"] = rec
	h.SetSession(456, sess)

	h.HandleCallback(context.Background(), testCallback(123, 456, "bolus:save"))

	require.Len(t, bot.sent, 1)
	msg := bot.lastMessage()
	assert.Contains(t, msg.Text, "Инсулин записан")
}

// TestBolusDetails_ShowsBreakdown verifies that the bolus:details callback
// returns a detailed breakdown of the recommendation.
func TestBolusDetails_ShowsBreakdown(t *testing.T) {
	bot := &spyBot{}
	loc := newTestLocalizer(t)

	rec := &domain.BolusRecommendation{
		TotalDose:      5.0,
		FoodDose:       4.0,
		CorrectionDose: 1.0,
		IOB:            0.5,
		ICR:            12.0,
		ISF:            2.5,
	}

	h := newTestHandler(bot, nil, nil, nil, nil, nil, nil, nil, nil, nil, loc)

	sess := telegram.NewSession(telegram.SessionBolus, telegram.StepBolusCarbs)
	sess.Data["recommendation"] = rec
	h.SetSession(456, sess)

	h.HandleCallback(context.Background(), testCallback(123, 456, "bolus:details"))

	require.Len(t, bot.sent, 1)
	text := bot.lastMessage().Text
	assert.Contains(t, text, "На еду")
	assert.Contains(t, text, "Коррекция")
	assert.Contains(t, text, "IOB")
	assert.Contains(t, text, "5")
}

// TestBolusCancel_ClearsSessionAndShowsMenu verifies that the bolus:cancel callback
// clears the session and returns the user to the main menu.
func TestBolusCancel_ClearsSessionAndShowsMenu(t *testing.T) {
	bot := &spyBot{}
	loc := newTestLocalizer(t)

	h := newTestHandler(bot, nil, nil, nil, nil, nil, nil, nil, nil, nil, loc)

	sess := telegram.NewSession(telegram.SessionBolus, telegram.StepBolusCarbs)
	sess.Data["user_id"] = int64(1)
	h.SetSession(456, sess)

	h.HandleCallback(context.Background(), testCallback(123, 456, "bolus:cancel"))

	require.Len(t, bot.sent, 1)
	text := bot.lastMessage().Text
	assert.Contains(t, text, "Выбери действие")
}
