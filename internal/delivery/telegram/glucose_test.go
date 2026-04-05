package telegram_test

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	telegram "github.com/gmtantsevov/shuggabuddy/internal/delivery/telegram"
	tgmocks "github.com/gmtantsevov/shuggabuddy/internal/delivery/telegram/mocks"
	"github.com/gmtantsevov/shuggabuddy/internal/domain"
)

func startGlucoseSession(h *telegram.Handler, chatID int64) {
	sess := telegram.NewSession(telegram.SessionGlucose, telegram.StepGlucoseValue)
	h.SetSession(chatID, sess)
}

// TestHandleGlucoseStep_InRangeIndicator verifies that saving an in-range reading appends 🟢.
func TestHandleGlucoseStep_InRangeIndicator(t *testing.T) {
	ctrl := gomock.NewController(t)
	bot := &spyBot{}
	userUC := tgmocks.NewMockUserUseCase(ctrl)
	glucUC := tgmocks.NewMockGlucoseUseCase(ctrl)
	loc := newTestLocalizer(t)

	h := newTestHandler(bot, userUC, glucUC, nil, nil, nil, nil, nil, nil, loc)
	startGlucoseSession(h, 100)

	userUC.EXPECT().GetProfile(gomock.Any(), domain.ProviderTelegram, "123").
		Return(testUser, testAcc, nil)
	glucUC.EXPECT().SaveReading(gomock.Any(), int64(1), 5.4, domain.UnitsMmol).
		Return(nil)

	sess := telegram.NewSession(telegram.SessionGlucose, telegram.StepGlucoseValue)
	h.HandleSessionInput(context.Background(), testMessage(123, 100, "5.4"), sess)

	require.GreaterOrEqual(t, len(bot.sent), 1)
	msg := bot.lastMessage()
	assert.Contains(t, msg.Text, "🟢")
}

// TestHandleGlucoseStep_LowIndicator verifies that a low reading appends 🔴.
func TestHandleGlucoseStep_LowIndicator(t *testing.T) {
	ctrl := gomock.NewController(t)
	bot := &spyBot{}
	userUC := tgmocks.NewMockUserUseCase(ctrl)
	glucUC := tgmocks.NewMockGlucoseUseCase(ctrl)
	loc := newTestLocalizer(t)

	h := newTestHandler(bot, userUC, glucUC, nil, nil, nil, nil, nil, nil, loc)

	userUC.EXPECT().GetProfile(gomock.Any(), domain.ProviderTelegram, "123").
		Return(testUser, testAcc, nil)
	glucUC.EXPECT().SaveReading(gomock.Any(), int64(1), 2.5, domain.UnitsMmol).
		Return(nil)

	sess := telegram.NewSession(telegram.SessionGlucose, telegram.StepGlucoseValue)
	h.HandleSessionInput(context.Background(), testMessage(123, 100, "2.5"), sess)

	require.GreaterOrEqual(t, len(bot.sent), 1)
	msg := bot.lastMessage()
	assert.Contains(t, msg.Text, "🔴")
}

// TestHandleGlucoseStep_HighIndicator verifies that a high reading appends 🟡.
func TestHandleGlucoseStep_HighIndicator(t *testing.T) {
	ctrl := gomock.NewController(t)
	bot := &spyBot{}
	userUC := tgmocks.NewMockUserUseCase(ctrl)
	glucUC := tgmocks.NewMockGlucoseUseCase(ctrl)
	loc := newTestLocalizer(t)

	h := newTestHandler(bot, userUC, glucUC, nil, nil, nil, nil, nil, nil, loc)

	userUC.EXPECT().GetProfile(gomock.Any(), domain.ProviderTelegram, "123").
		Return(testUser, testAcc, nil)
	glucUC.EXPECT().SaveReading(gomock.Any(), int64(1), 14.0, domain.UnitsMmol).
		Return(nil)

	sess := telegram.NewSession(telegram.SessionGlucose, telegram.StepGlucoseValue)
	h.HandleSessionInput(context.Background(), testMessage(123, 100, "14.0"), sess)

	require.GreaterOrEqual(t, len(bot.sent), 1)
	msg := bot.lastMessage()
	assert.Contains(t, msg.Text, "🟡")
}

// TestHandleGlucoseStep_InvalidInput_NoIndicator verifies that invalid input sends no indicator.
func TestHandleGlucoseStep_InvalidInput_NoIndicator(t *testing.T) {
	bot := &spyBot{}
	loc := newTestLocalizer(t)

	h := newTestHandler(bot, nil, nil, nil, nil, nil, nil, nil, nil, loc)

	sess := telegram.NewSession(telegram.SessionGlucose, telegram.StepGlucoseValue)
	h.HandleSessionInput(context.Background(), testMessage(123, 100, "abc"), sess)

	require.GreaterOrEqual(t, len(bot.sent), 1)
	msg := bot.lastMessage()
	assert.True(t, !strings.Contains(msg.Text, "🟢") && !strings.Contains(msg.Text, "🔴") && !strings.Contains(msg.Text, "🟡"))
}
