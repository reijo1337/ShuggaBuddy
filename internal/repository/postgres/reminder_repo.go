package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gmtantsevov/shuggabuddy/internal/domain"
)

type ReminderRepo struct {
	pool *pgxpool.Pool
}

func NewReminderRepo(pool *pgxpool.Pool) *ReminderRepo {
	return &ReminderRepo{pool: pool}
}

func (r *ReminderRepo) Save(ctx context.Context, reminder *domain.Reminder) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO reminders (user_id, activity_id, chat_id, fire_at)
		 VALUES ($1, $2, $3, $4)`,
		reminder.UserID, reminder.ActivityID, reminder.ChatID, reminder.FireAt,
	)
	if err != nil {
		return fmt.Errorf("ReminderRepo.Save: %w", err)
	}
	return nil
}

func (r *ReminderRepo) GetPending(ctx context.Context, now time.Time) ([]domain.Reminder, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, user_id, activity_id, chat_id, fire_at, fired, created_at
		 FROM reminders
		 WHERE fired = false AND fire_at <= $1
		 ORDER BY fire_at ASC`,
		now,
	)
	if err != nil {
		return nil, fmt.Errorf("ReminderRepo.GetPending: %w", err)
	}
	defer rows.Close()

	var reminders []domain.Reminder
	for rows.Next() {
		var rm domain.Reminder
		if err := rows.Scan(&rm.ID, &rm.UserID, &rm.ActivityID, &rm.ChatID, &rm.FireAt, &rm.Fired, &rm.CreatedAt); err != nil {
			return nil, fmt.Errorf("ReminderRepo.GetPending: scan: %w", err)
		}
		reminders = append(reminders, rm)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ReminderRepo.GetPending: rows: %w", err)
	}
	return reminders, nil
}

func (r *ReminderRepo) MarkFired(ctx context.Context, id int64) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE reminders SET fired = true WHERE id = $1`,
		id,
	)
	if err != nil {
		return fmt.Errorf("ReminderRepo.MarkFired: %w", err)
	}
	return nil
}
