package scheduler

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/gmtantsevov/shuggabuddy/internal/domain"
)

// Messenger отправляет текстовые сообщения в чат.
type Messenger interface {
	SendReminder(chatID int64, text string) error
}

// ReminderMessage используется для тестирования.
type ReminderMessage struct {
	ChatID int64
	Text   string
}

// ReminderScheduler опрашивает БД на предмет созревших напоминаний.
type ReminderScheduler struct {
	reminderRepo domain.ReminderRepository
	activityRepo domain.ActivityRepository
	glucoseRepo  domain.GlucoseRepository
	messenger    Messenger
	log          *zap.Logger
}

func NewReminderScheduler(
	reminderRepo domain.ReminderRepository,
	activityRepo domain.ActivityRepository,
	glucoseRepo domain.GlucoseRepository,
	messenger Messenger,
	log *zap.Logger,
) *ReminderScheduler {
	return &ReminderScheduler{
		reminderRepo: reminderRepo,
		activityRepo: activityRepo,
		glucoseRepo:  glucoseRepo,
		messenger:    messenger,
		log:          log,
	}
}

// Run запускает цикл опроса с интервалом 30 секунд.
// Обрабатывает накопившиеся напоминания сразу при старте.
func (s *ReminderScheduler) Run(ctx context.Context) {
	s.ProcessPending(ctx)

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.ProcessPending(ctx)
		}
	}
}

// ProcessPending обрабатывает все созревшие напоминания.
func (s *ReminderScheduler) ProcessPending(ctx context.Context) {
	reminders, err := s.reminderRepo.GetPending(ctx, time.Now())
	if err != nil {
		s.log.Error("scheduler: failed to get pending reminders", zap.Error(err))
		return
	}

	for _, r := range reminders {
		s.processOne(ctx, r)
	}
}

func (s *ReminderScheduler) processOne(ctx context.Context, r domain.Reminder) {
	entry, err := s.activityRepo.GetByID(ctx, r.ActivityID)
	if err != nil {
		s.log.Error("scheduler: failed to get activity", zap.Error(err), zap.Int64("activity_id", r.ActivityID))
		return
	}

	beforeReadings, err := s.glucoseRepo.GetByTimeRange(ctx, r.UserID, entry.RecordedAt.Add(-1*time.Hour), entry.RecordedAt)
	if err != nil {
		s.log.Error("scheduler: failed to get glucose before", zap.Error(err))
		return
	}

	afterReadings, err := s.glucoseRepo.GetByTimeRange(ctx, r.UserID, entry.RecordedAt, time.Now())
	if err != nil {
		s.log.Error("scheduler: failed to get glucose after", zap.Error(err))
		return
	}

	text := s.formatMessage(entry, beforeReadings, afterReadings)

	if err := s.messenger.SendReminder(r.ChatID, text); err != nil {
		s.log.Error("scheduler: failed to send reminder", zap.Error(err), zap.Int64("chat_id", r.ChatID))
		return
	}

	if err := s.reminderRepo.MarkFired(ctx, r.ID); err != nil {
		s.log.Error("scheduler: failed to mark fired", zap.Error(err), zap.Int64("reminder_id", r.ID))
	}
}

func (s *ReminderScheduler) formatMessage(entry *domain.ActivityEntry, before, after []domain.GlucoseReading) string {
	typeLabel := string(entry.ActivityType)
	if entry.CustomType != "" {
		typeLabel = entry.CustomType
	}

	if len(before) > 0 && len(after) > 0 {
		bVal := before[len(before)-1].ValueMmol
		aVal := after[len(after)-1].ValueMmol
		delta := aVal - bVal
		sign := "+"
		if delta < 0 {
			sign = ""
		}
		return fmt.Sprintf("📊 Отчёт по активности\n%s · %d мин\n🩸 До: %.1f → После: %.1f (%s%.1f)",
			typeLabel, entry.DurationMin, bVal, aVal, sign, delta)
	}

	return fmt.Sprintf("⏰ Пора замерить сахар!\nПосле активности: %s · %d мин",
		typeLabel, entry.DurationMin)
}
