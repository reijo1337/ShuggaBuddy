package domain

import (
	"context"
	"time"
)

// Reminder представляет отложенное напоминание о замере глюкозы после активности.
type Reminder struct {
	ID         int64     `json:"id"`
	UserID     int64     `json:"user_id"`
	ActivityID int64     `json:"activity_id"`
	ChatID     int64     `json:"chat_id"`
	FireAt     time.Time `json:"fire_at"`
	Fired      bool      `json:"fired"`
	CreatedAt  time.Time `json:"created_at"`
}

//go:generate mockgen -destination=mocks/mock_reminder_repository.go -package=mocks . ReminderRepository

// ReminderRepository описывает хранилище напоминаний.
type ReminderRepository interface {
	Save(ctx context.Context, reminder *Reminder) error
	GetPending(ctx context.Context, now time.Time) ([]Reminder, error)
	MarkFired(ctx context.Context, id int64) error
}
