package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gmtantsevov/shuggabuddy/internal/domain"
)

type ActivityRepo struct {
	pool *pgxpool.Pool
}

func NewActivityRepo(pool *pgxpool.Pool) *ActivityRepo {
	return &ActivityRepo{pool: pool}
}

func (r *ActivityRepo) Save(ctx context.Context, entry *domain.ActivityEntry) (int64, error) {
	var id int64
	err := r.pool.QueryRow(ctx,
		`INSERT INTO activity_entries (user_id, activity_type, custom_type, duration_min, intensity, recorded_at)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id`,
		entry.UserID, entry.ActivityType, entry.CustomType, entry.DurationMin, entry.Intensity, entry.RecordedAt,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("ActivityRepo.Save: %w", err)
	}
	return id, nil
}

func (r *ActivityRepo) GetByID(ctx context.Context, id int64) (*domain.ActivityEntry, error) {
	var e domain.ActivityEntry
	err := r.pool.QueryRow(ctx,
		`SELECT id, user_id, activity_type, custom_type, duration_min, intensity, recorded_at, created_at
		 FROM activity_entries
		 WHERE id = $1`,
		id,
	).Scan(&e.ID, &e.UserID, &e.ActivityType, &e.CustomType, &e.DurationMin, &e.Intensity, &e.RecordedAt, &e.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("ActivityRepo.GetByID: %w", err)
	}
	return &e, nil
}

func (r *ActivityRepo) GetLast(ctx context.Context, userID int64, limit int) ([]domain.ActivityEntry, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, user_id, activity_type, custom_type, duration_min, intensity, recorded_at, created_at
		 FROM activity_entries
		 WHERE user_id = $1
		 ORDER BY recorded_at DESC
		 LIMIT $2`,
		userID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("ActivityRepo.GetLast: %w", err)
	}
	defer rows.Close()

	var entries []domain.ActivityEntry
	for rows.Next() {
		var e domain.ActivityEntry
		if err := rows.Scan(&e.ID, &e.UserID, &e.ActivityType, &e.CustomType, &e.DurationMin, &e.Intensity, &e.RecordedAt, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("ActivityRepo.GetLast: scan: %w", err)
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ActivityRepo.GetLast: rows: %w", err)
	}
	return entries, nil
}

func (r *ActivityRepo) GetByTimeRange(ctx context.Context, userID int64, from, to time.Time) ([]*domain.ActivityEntry, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, user_id, activity_type, custom_type, duration_min, intensity, recorded_at, created_at
		 FROM activity_entries
		 WHERE user_id = $1 AND recorded_at >= $2 AND recorded_at < $3
		 ORDER BY recorded_at DESC`,
		userID, from, to,
	)
	if err != nil {
		return nil, fmt.Errorf("ActivityRepo.GetByTimeRange: %w", err)
	}
	defer rows.Close()

	var entries []*domain.ActivityEntry
	for rows.Next() {
		e := &domain.ActivityEntry{}
		if err := rows.Scan(&e.ID, &e.UserID, &e.ActivityType, &e.CustomType, &e.DurationMin, &e.Intensity, &e.RecordedAt, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("ActivityRepo.GetByTimeRange: scan: %w", err)
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ActivityRepo.GetByTimeRange: rows: %w", err)
	}
	return entries, nil
}
