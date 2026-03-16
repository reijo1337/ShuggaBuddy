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
	"github.com/gmtantsevov/shuggabuddy/internal/scheduler"
)

type spyMessenger struct {
	messages []scheduler.ReminderMessage
}

func (s *spyMessenger) SendReminder(chatID int64, text string) error {
	s.messages = append(s.messages, scheduler.ReminderMessage{ChatID: chatID, Text: text})
	return nil
}

func TestScheduler_ProcessPending_WithGlucose(t *testing.T) {
	ctrl := gomock.NewController(t)
	reminderRepo := mocks.NewMockReminderRepository(ctrl)
	activityRepo := mocks.NewMockActivityRepository(ctrl)
	glucoseRepo := mocks.NewMockGlucoseRepository(ctrl)
	messenger := &spyMessenger{}

	now := time.Date(2026, 3, 16, 12, 0, 0, 0, time.UTC)
	activityTime := now.Add(-2 * time.Hour)

	pending := []domain.Reminder{
		{ID: 1, UserID: 1, ActivityID: 10, ChatID: 456, FireAt: now.Add(-1 * time.Minute)},
	}

	entry := &domain.ActivityEntry{
		ID: 10, UserID: 1, ActivityType: domain.ActivityRunning,
		DurationMin: 30, Intensity: domain.IntensityMedium, RecordedAt: activityTime,
	}

	glucBefore := []domain.GlucoseReading{
		{ValueMmol: 6.0, RecordedAt: activityTime.Add(-10 * time.Minute)},
	}
	glucAfter := []domain.GlucoseReading{
		{ValueMmol: 4.5, RecordedAt: activityTime.Add(90 * time.Minute)},
	}

	reminderRepo.EXPECT().GetPending(gomock.Any(), gomock.Any()).Return(pending, nil)
	activityRepo.EXPECT().GetByID(gomock.Any(), int64(10)).Return(entry, nil)
	glucoseRepo.EXPECT().GetByTimeRange(gomock.Any(), int64(1), activityTime.Add(-1*time.Hour), activityTime).Return(glucBefore, nil)
	glucoseRepo.EXPECT().GetByTimeRange(gomock.Any(), int64(1), activityTime, gomock.Any()).Return(glucAfter, nil)
	reminderRepo.EXPECT().MarkFired(gomock.Any(), int64(1)).Return(nil)

	s := scheduler.NewReminderScheduler(reminderRepo, activityRepo, glucoseRepo, messenger, zap.NewNop())
	s.ProcessPending(context.Background())

	assert.Len(t, messenger.messages, 1)
	assert.Equal(t, int64(456), messenger.messages[0].ChatID)
	assert.Contains(t, messenger.messages[0].Text, "6.0")
	assert.Contains(t, messenger.messages[0].Text, "4.5")
}

func TestScheduler_ProcessPending_NoGlucose(t *testing.T) {
	ctrl := gomock.NewController(t)
	reminderRepo := mocks.NewMockReminderRepository(ctrl)
	activityRepo := mocks.NewMockActivityRepository(ctrl)
	glucoseRepo := mocks.NewMockGlucoseRepository(ctrl)
	messenger := &spyMessenger{}

	now := time.Date(2026, 3, 16, 12, 0, 0, 0, time.UTC)
	activityTime := now.Add(-1 * time.Hour)

	pending := []domain.Reminder{
		{ID: 2, UserID: 1, ActivityID: 20, ChatID: 789, FireAt: now},
	}
	entry := &domain.ActivityEntry{
		ID: 20, UserID: 1, ActivityType: domain.ActivityYoga,
		DurationMin: 30, Intensity: domain.IntensityMedium, RecordedAt: activityTime,
	}

	reminderRepo.EXPECT().GetPending(gomock.Any(), gomock.Any()).Return(pending, nil)
	activityRepo.EXPECT().GetByID(gomock.Any(), int64(20)).Return(entry, nil)
	glucoseRepo.EXPECT().GetByTimeRange(gomock.Any(), int64(1), gomock.Any(), gomock.Any()).Return(nil, nil).Times(2)
	reminderRepo.EXPECT().MarkFired(gomock.Any(), int64(2)).Return(nil)

	s := scheduler.NewReminderScheduler(reminderRepo, activityRepo, glucoseRepo, messenger, zap.NewNop())
	s.ProcessPending(context.Background())

	assert.Len(t, messenger.messages, 1)
	assert.Equal(t, int64(789), messenger.messages[0].ChatID)
	assert.Contains(t, messenger.messages[0].Text, "замерить")
}

func TestScheduler_ProcessPending_NoPending(t *testing.T) {
	ctrl := gomock.NewController(t)
	reminderRepo := mocks.NewMockReminderRepository(ctrl)
	messenger := &spyMessenger{}

	reminderRepo.EXPECT().GetPending(gomock.Any(), gomock.Any()).Return(nil, nil)

	s := scheduler.NewReminderScheduler(reminderRepo, nil, nil, messenger, zap.NewNop())
	s.ProcessPending(context.Background())

	assert.Empty(t, messenger.messages)
}
