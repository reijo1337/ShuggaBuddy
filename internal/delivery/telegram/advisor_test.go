package telegram_test

import (
	"context"
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

func TestHandleAdvisorShow_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	bot := &spyBot{}
	loc := newTestLocalizer(t)
	userUC := tgmocks.NewMockUserUseCase(ctrl)
	advisorUC := tgmocks.NewMockDoseAdvisorUseCase(ctrl)

	user := &domain.User{
		ID: 1, BasalDose: 18, TargetMinMmol: 3.9, TargetMaxMmol: 10.0,
	}
	acc := &domain.ExternalAccount{ExternalID: "111"}

	userUC.EXPECT().GetProfile(gomock.Any(), domain.ProviderTelegram, "111").Return(user, acc, nil)

	advice := &domain.DoseAdvice{
		AnalyzedAt: time.Now(),
		Basal: &domain.BasalAdvice{
			Trend: domain.TrendHigh, FastingAvg: 12.0, FastingCount: 7,
			CurrentDose: 18, SuggestedDose: 20, TargetMin: 3.9, TargetMax: 10.0,
		},
	}
	advisorUC.EXPECT().Analyze(gomock.Any(), int64(1), gomock.Any()).Return(advice, nil)

	h := newTestHandler(bot, userUC, nil, nil, nil, nil, nil, nil, nil, advisorUC, loc)
	h.HandleCallback(context.Background(), testCallback(111, 222, "menu:advisor"))

	require.Len(t, bot.sent, 1)
	msg := bot.sent[0].(tgbotapi.MessageConfig)
	assert.Contains(t, msg.Text, "12.0")
}

func TestHandleAdvisorShow_NoData(t *testing.T) {
	ctrl := gomock.NewController(t)
	bot := &spyBot{}
	loc := newTestLocalizer(t)
	userUC := tgmocks.NewMockUserUseCase(ctrl)
	advisorUC := tgmocks.NewMockDoseAdvisorUseCase(ctrl)

	user := &domain.User{ID: 1, BasalDose: 0}
	acc := &domain.ExternalAccount{ExternalID: "111"}

	userUC.EXPECT().GetProfile(gomock.Any(), domain.ProviderTelegram, "111").Return(user, acc, nil)
	advisorUC.EXPECT().Analyze(gomock.Any(), int64(1), gomock.Any()).Return(&domain.DoseAdvice{AnalyzedAt: time.Now()}, nil)

	h := newTestHandler(bot, userUC, nil, nil, nil, nil, nil, nil, nil, advisorUC, loc)
	h.HandleCallback(context.Background(), testCallback(111, 222, "menu:advisor"))

	require.Len(t, bot.sent, 1)
}

func TestHandleAdvisorShow_AnalysisError(t *testing.T) {
	ctrl := gomock.NewController(t)
	bot := &spyBot{}
	loc := newTestLocalizer(t)
	userUC := tgmocks.NewMockUserUseCase(ctrl)
	advisorUC := tgmocks.NewMockDoseAdvisorUseCase(ctrl)

	user := &domain.User{ID: 1}
	acc := &domain.ExternalAccount{ExternalID: "111"}

	userUC.EXPECT().GetProfile(gomock.Any(), domain.ProviderTelegram, "111").Return(user, acc, nil)
	advisorUC.EXPECT().Analyze(gomock.Any(), int64(1), gomock.Any()).Return(nil, assert.AnError)

	h := newTestHandler(bot, userUC, nil, nil, nil, nil, nil, nil, nil, advisorUC, loc)
	h.HandleCallback(context.Background(), testCallback(111, 222, "menu:advisor"))

	require.Len(t, bot.sent, 1)
	msg := bot.sent[0].(tgbotapi.MessageConfig)
	assert.Contains(t, msg.Text, loc.T("error_internal"))
}

func TestHandleProfileBasalDoseInput_Valid(t *testing.T) {
	ctrl := gomock.NewController(t)
	bot := &spyBot{}
	loc := newTestLocalizer(t)
	userUC := tgmocks.NewMockUserUseCase(ctrl)

	user := &domain.User{ID: 1}
	acc := &domain.ExternalAccount{ExternalID: "111"}

	userUC.EXPECT().GetProfile(gomock.Any(), domain.ProviderTelegram, "111").Return(user, acc, nil)
	userUC.EXPECT().UpdateBasalDose(gomock.Any(), int64(1), 18.5).Return(nil)

	h := newTestHandler(bot, userUC, nil, nil, nil, nil, nil, nil, nil, nil, loc)

	// Start the basal dose session
	h.HandleCallback(context.Background(), testCallback(111, 222, "profile:basal_dose"))
	require.Len(t, bot.sent, 1)

	// Input dose value
	sess := telegram.NewSession(telegram.SessionProfile, telegram.StepProfileBasalDose)
	h.HandleSessionInput(context.Background(), testMessage(111, 222, "18.5"), sess)

	require.Len(t, bot.sent, 2)
}

func TestHandleProfileBasalDoseInput_Invalid(t *testing.T) {
	bot := &spyBot{}
	loc := newTestLocalizer(t)

	h := newTestHandler(bot, nil, nil, nil, nil, nil, nil, nil, nil, nil, loc)

	h.HandleCallback(context.Background(), testCallback(111, 222, "profile:basal_dose"))

	sess := telegram.NewSession(telegram.SessionProfile, telegram.StepProfileBasalDose)
	h.HandleSessionInput(context.Background(), testMessage(111, 222, "abc"), sess)

	require.Len(t, bot.sent, 2)
}

func TestHandleAdvisorIntervalSet(t *testing.T) {
	ctrl := gomock.NewController(t)
	bot := &spyBot{}
	loc := newTestLocalizer(t)
	userUC := tgmocks.NewMockUserUseCase(ctrl)

	user := &domain.User{ID: 1}
	acc := &domain.ExternalAccount{ExternalID: "111"}

	userUC.EXPECT().GetProfile(gomock.Any(), domain.ProviderTelegram, "111").Return(user, acc, nil)
	userUC.EXPECT().UpdateAdvisorInterval(gomock.Any(), int64(1), 7).Return(nil)

	h := newTestHandler(bot, userUC, nil, nil, nil, nil, nil, nil, nil, nil, loc)
	h.HandleCallback(context.Background(), testCallback(111, 222, "profile:advisor_interval:7"))

	require.Len(t, bot.sent, 1)
}

func TestHandleAdvisorIntervalOff(t *testing.T) {
	ctrl := gomock.NewController(t)
	bot := &spyBot{}
	loc := newTestLocalizer(t)
	userUC := tgmocks.NewMockUserUseCase(ctrl)

	user := &domain.User{ID: 1}
	acc := &domain.ExternalAccount{ExternalID: "111"}

	userUC.EXPECT().GetProfile(gomock.Any(), domain.ProviderTelegram, "111").Return(user, acc, nil)
	userUC.EXPECT().UpdateAdvisorInterval(gomock.Any(), int64(1), 0).Return(nil)

	h := newTestHandler(bot, userUC, nil, nil, nil, nil, nil, nil, nil, nil, loc)
	h.HandleCallback(context.Background(), testCallback(111, 222, "profile:advisor_interval:off"))

	require.Len(t, bot.sent, 1)
}
