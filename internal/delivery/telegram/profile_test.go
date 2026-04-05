package telegram_test

import (
	"context"
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/gmtantsevov/shuggabuddy/internal/delivery/telegram"
	tgmocks "github.com/gmtantsevov/shuggabuddy/internal/delivery/telegram/mocks"
	"github.com/gmtantsevov/shuggabuddy/internal/domain"
)

// --- Test 1: handleProfileCb shows target range ---

func TestHandleProfileCb_ShowsTargetRange(t *testing.T) {
	ctrl := gomock.NewController(t)
	bot := &spyBot{}
	userUC := tgmocks.NewMockUserUseCase(ctrl)
	loc := newTestLocalizer(t)

	user := &domain.User{
		ID:            1,
		Units:         domain.UnitsMmol,
		CarbsPerUnit:  12,
		TargetMinMmol: 3.9,
		TargetMaxMmol: 10.0,
		CreatedAt:     testUser.CreatedAt,
	}
	acc := &domain.ExternalAccount{DisplayName: "Test"}
	userUC.EXPECT().GetProfile(gomock.Any(), domain.ProviderTelegram, "123").Return(user, acc, nil)

	h := newTestHandler(bot, userUC, nil, nil, nil, nil, nil, nil, nil, loc)
	h.HandleCallback(context.Background(), testCallback(123, 456, "menu:profile"))

	require.Len(t, bot.sent, 1)
	text := bot.lastMessage().Text
	assert.Contains(t, text, "3.9")
	assert.Contains(t, text, "10.0")
}

// --- Test 2: target range valid flow ---

func TestHandleProfileTargetRange_ValidFlow(t *testing.T) {
	ctrl := gomock.NewController(t)
	bot := &spyBot{}
	userUC := tgmocks.NewMockUserUseCase(ctrl)
	loc := newTestLocalizer(t)

	// Step 1: start target range flow
	h := newTestHandler(bot, userUC, nil, nil, nil, nil, nil, nil, nil, loc)
	h.HandleCallback(context.Background(), testCallback(123, 456, "profile:target_range"))

	require.Len(t, bot.sent, 1)
	assert.Contains(t, bot.lastMessage().Text, "нижнюю")

	// Step 2: enter min
	bot.sent = nil
	sess := telegram.NewSession(telegram.SessionProfile, telegram.StepProfileTargetMin)
	h.SetSession(456, sess)
	h.HandleSessionInput(context.Background(), testMessage(123, 456, "3.9"), sess)

	require.Len(t, bot.sent, 1)
	assert.Contains(t, bot.lastMessage().Text, "верхнюю")

	// Step 3: enter max — expect UpdateSettings to be called
	bot.sent = nil
	userUC.EXPECT().GetProfile(gomock.Any(), domain.ProviderTelegram, "123").Return(testUser, testAcc, nil)
	userUC.EXPECT().UpdateSettings(gomock.Any(), int64(1), 3.9, 10.0, testUser.BasalDrug, testUser.BasalTime).Return(nil)

	// Re-load session from handler (it was stored after entering min)
	h.HandleSessionInput(context.Background(), testMessage(123, 456, "10.0"), sess)

	require.Len(t, bot.sent, 1)
	assert.Contains(t, bot.lastMessage().Text, "обновлён")
}

// --- Test 3: invalid min value ---

func TestHandleProfileTargetRange_InvalidMin(t *testing.T) {
	ctrl := gomock.NewController(t)
	bot := &spyBot{}
	userUC := tgmocks.NewMockUserUseCase(ctrl)
	loc := newTestLocalizer(t)

	h := newTestHandler(bot, userUC, nil, nil, nil, nil, nil, nil, nil, loc)
	sess := telegram.NewSession(telegram.SessionProfile, telegram.StepProfileTargetMin)
	h.SetSession(456, sess)

	h.HandleSessionInput(context.Background(), testMessage(123, 456, "0.5"), sess)

	require.Len(t, bot.sent, 1)
	assert.Contains(t, bot.lastMessage().Text, "Некорректное значение")

	// Session should NOT be cleared — still has profile type
	// (no call to UpdateSettings expected)
}

// --- Test 4: max less than min ---

func TestHandleProfileTargetRange_MaxLessThanMin(t *testing.T) {
	ctrl := gomock.NewController(t)
	bot := &spyBot{}
	userUC := tgmocks.NewMockUserUseCase(ctrl)
	loc := newTestLocalizer(t)

	h := newTestHandler(bot, userUC, nil, nil, nil, nil, nil, nil, nil, loc)

	// Simulate having entered min=5.0
	sess := telegram.NewSession(telegram.SessionProfile, telegram.StepProfileTargetMax)
	sess.Data["target_min"] = 5.0
	h.SetSession(456, sess)

	h.HandleSessionInput(context.Background(), testMessage(123, 456, "4.0"), sess)

	require.Len(t, bot.sent, 1)
	assert.Contains(t, bot.lastMessage().Text, "больше нижней")
}

// --- Test 5: basal valid flow ---

func TestHandleProfileBasal_ValidFlow(t *testing.T) {
	ctrl := gomock.NewController(t)
	bot := &spyBot{}
	userUC := tgmocks.NewMockUserUseCase(ctrl)
	loc := newTestLocalizer(t)

	h := newTestHandler(bot, userUC, nil, nil, nil, nil, nil, nil, nil, loc)

	// Step 1: start basal flow
	h.HandleCallback(context.Background(), testCallback(123, 456, "profile:basal"))

	require.Len(t, bot.sent, 1)
	assert.Contains(t, bot.lastMessage().Text, "базальн")

	// Step 2: enter drug name
	bot.sent = nil
	sess := telegram.NewSession(telegram.SessionProfile, telegram.StepProfileBasalDrug)
	h.SetSession(456, sess)
	h.HandleSessionInput(context.Background(), testMessage(123, 456, "Lantus"), sess)

	require.Len(t, bot.sent, 1)
	assert.Contains(t, bot.lastMessage().Text, "время")

	// Step 3: enter time — expect UpdateSettings to be called
	bot.sent = nil
	userUC.EXPECT().GetProfile(gomock.Any(), domain.ProviderTelegram, "123").Return(testUser, testAcc, nil)
	userUC.EXPECT().UpdateSettings(gomock.Any(), int64(1), testUser.TargetMinMmol, testUser.TargetMaxMmol, "Lantus", "22:00").Return(nil)

	h.HandleSessionInput(context.Background(), testMessage(123, 456, "22:00"), sess)

	require.Len(t, bot.sent, 1)
	assert.Contains(t, bot.lastMessage().Text, "сохранён")
}

// --- Test 6: basal skip drug + skip time ---

func TestHandleProfileBasal_SkipDrug_SkipTime(t *testing.T) {
	ctrl := gomock.NewController(t)
	bot := &spyBot{}
	userUC := tgmocks.NewMockUserUseCase(ctrl)
	loc := newTestLocalizer(t)

	h := newTestHandler(bot, userUC, nil, nil, nil, nil, nil, nil, nil, loc)

	// Start basal flow
	h.HandleCallback(context.Background(), testCallback(123, 456, "profile:basal"))
	bot.sent = nil

	// Skip drug
	sess := telegram.NewSession(telegram.SessionProfile, telegram.StepProfileBasalDrug)
	h.SetSession(456, sess)
	h.HandleCallback(context.Background(), testCallback(123, 456, "profile:basal:skip_drug"))

	require.Len(t, bot.sent, 1)
	assert.Contains(t, bot.lastMessage().Text, "время")

	// Skip time — expect UpdateSettings to be called with ("", "")
	bot.sent = nil
	userUC.EXPECT().GetProfile(gomock.Any(), domain.ProviderTelegram, "123").Return(testUser, testAcc, nil)
	userUC.EXPECT().UpdateSettings(gomock.Any(), int64(1), testUser.TargetMinMmol, testUser.TargetMaxMmol, "", "").Return(nil)

	h.HandleCallback(context.Background(), testCallback(123, 456, "profile:basal:skip_time"))

	require.Len(t, bot.sent, 1)
	assert.Contains(t, bot.lastMessage().Text, "сохранён")
}

// --- Test 7: invalid time re-prompts ---

func TestHandleProfileBasal_InvalidTime(t *testing.T) {
	ctrl := gomock.NewController(t)
	bot := &spyBot{}
	userUC := tgmocks.NewMockUserUseCase(ctrl)
	loc := newTestLocalizer(t)

	h := newTestHandler(bot, userUC, nil, nil, nil, nil, nil, nil, nil, loc)

	sess := telegram.NewSession(telegram.SessionProfile, telegram.StepProfileBasalTime)
	sess.Data["basal_drug"] = "Lantus"
	h.SetSession(456, sess)

	h.HandleSessionInput(context.Background(), testMessage(123, 456, "25:00"), sess)

	require.Len(t, bot.sent, 1)
	// Re-prompt: should contain time prompt text
	assert.Contains(t, bot.lastMessage().Text, "время")

	// UpdateSettings must NOT have been called (no EXPECT set up, so ctrl would fail if called)
}

// --- Test: profile keyboard has target_range and basal buttons ---

func TestHandleProfileCb_KeyboardHasNewButtons(t *testing.T) {
	ctrl := gomock.NewController(t)
	bot := &spyBot{}
	userUC := tgmocks.NewMockUserUseCase(ctrl)
	loc := newTestLocalizer(t)

	userUC.EXPECT().GetProfile(gomock.Any(), domain.ProviderTelegram, "123").Return(testUser, testAcc, nil)

	h := newTestHandler(bot, userUC, nil, nil, nil, nil, nil, nil, nil, loc)
	h.HandleCallback(context.Background(), testCallback(123, 456, "menu:profile"))

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
	assert.Contains(t, callbacks, "profile:target_range")
	assert.Contains(t, callbacks, "profile:basal")
	assert.Contains(t, callbacks, "menu:units")
	assert.Contains(t, callbacks, "menu:carbs_unit")
}
