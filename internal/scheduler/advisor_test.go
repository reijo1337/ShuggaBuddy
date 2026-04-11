package scheduler_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"

	"github.com/gmtantsevov/shuggabuddy/internal/domain"
	"github.com/gmtantsevov/shuggabuddy/internal/domain/mocks"
	"github.com/gmtantsevov/shuggabuddy/internal/i18n"
	"github.com/gmtantsevov/shuggabuddy/internal/scheduler"
)

type mockAdvisor struct {
	advice *domain.DoseAdvice
	err    error
}

func (m *mockAdvisor) Analyze(_ context.Context, _ int64, _ time.Time) (*domain.DoseAdvice, error) {
	return m.advice, m.err
}

func testLocalizer(t *testing.T) *i18n.Localizer {
	t.Helper()
	loc, err := i18n.NewLocalizer("../../locales", "ru")
	if err != nil {
		t.Fatalf("failed to init localizer: %v", err)
	}
	return loc
}

func TestAdvisorScheduler_ProcessPending_SendsAdvice(t *testing.T) {
	ctrl := gomock.NewController(t)
	userRepo := mocks.NewMockUserRepository(ctrl)
	extAccRepo := mocks.NewMockExternalAccountRepository(ctrl)
	messenger := &spyMessenger{}
	loc := testLocalizer(t)

	now := time.Now()
	users := []domain.User{
		{ID: 1, AdvisorIntervalDays: 7},
	}

	advice := &domain.DoseAdvice{
		AnalyzedAt: now,
		Basal: &domain.BasalAdvice{
			Trend: domain.TrendHigh, FastingAvg: 11.5, FastingCount: 6,
			CurrentDose: 16, SuggestedDose: 17,
		},
	}

	userRepo.EXPECT().GetUsersForAdvisor(gomock.Any(), gomock.Any()).Return(users, nil)
	extAccRepo.EXPECT().GetByUserID(gomock.Any(), int64(1), domain.ProviderTelegram).
		Return(&domain.ExternalAccount{ExternalID: "456"}, nil)
	userRepo.EXPECT().UpdateAdvisorLastSentAt(gomock.Any(), int64(1), gomock.Any()).Return(nil)

	advisorUC := &mockAdvisor{advice: advice}

	s := scheduler.NewAdvisorScheduler(userRepo, extAccRepo, advisorUC, messenger, loc, zap.NewNop())
	s.ProcessPending(context.Background())

	assert.Len(t, messenger.messages, 1)
	assert.Equal(t, int64(456), messenger.messages[0].ChatID)
	assert.Contains(t, messenger.messages[0].Text, "11.5")
}

func TestAdvisorScheduler_ProcessPending_SkipsNoData(t *testing.T) {
	ctrl := gomock.NewController(t)
	userRepo := mocks.NewMockUserRepository(ctrl)
	extAccRepo := mocks.NewMockExternalAccountRepository(ctrl)
	messenger := &spyMessenger{}
	loc := testLocalizer(t)

	users := []domain.User{
		{ID: 2, AdvisorIntervalDays: 7},
	}

	advice := &domain.DoseAdvice{AnalyzedAt: time.Now()} // no basal or bolus

	userRepo.EXPECT().GetUsersForAdvisor(gomock.Any(), gomock.Any()).Return(users, nil)

	advisorUC := &mockAdvisor{advice: advice}

	s := scheduler.NewAdvisorScheduler(userRepo, extAccRepo, advisorUC, messenger, loc, zap.NewNop())
	s.ProcessPending(context.Background())

	assert.Empty(t, messenger.messages)
}

func TestAdvisorScheduler_ProcessPending_NoPendingUsers(t *testing.T) {
	ctrl := gomock.NewController(t)
	userRepo := mocks.NewMockUserRepository(ctrl)
	extAccRepo := mocks.NewMockExternalAccountRepository(ctrl)
	messenger := &spyMessenger{}
	loc := testLocalizer(t)

	userRepo.EXPECT().GetUsersForAdvisor(gomock.Any(), gomock.Any()).Return(nil, nil)

	advisorUC := &mockAdvisor{}

	s := scheduler.NewAdvisorScheduler(userRepo, extAccRepo, advisorUC, messenger, loc, zap.NewNop())
	s.ProcessPending(context.Background())

	assert.Empty(t, messenger.messages)
}
