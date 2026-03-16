package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gmtantsevov/shuggabuddy/internal/domain"
)

type GlucoseRepo struct {
	pool *pgxpool.Pool
}

func NewGlucoseRepo(pool *pgxpool.Pool) *GlucoseRepo {
	return &GlucoseRepo{pool: pool}
}

func (r *GlucoseRepo) Save(ctx context.Context, reading *domain.GlucoseReading) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO glucose_readings (user_id, value_mmol, source)
		 VALUES ($1, $2, $3)`,
		reading.UserID, reading.ValueMmol, reading.Source,
	)
	if err != nil {
		return fmt.Errorf("GlucoseRepo.Save: %w", err)
	}

	return nil
}

func (r *GlucoseRepo) GetLast(ctx context.Context, userID int64, limit int) ([]domain.GlucoseReading, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, user_id, value_mmol, source, recorded_at
		 FROM glucose_readings
		 WHERE user_id = $1
		 ORDER BY recorded_at DESC
		 LIMIT $2`,
		userID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("GlucoseRepo.GetLast: %w", err)
	}
	defer rows.Close()

	var readings []domain.GlucoseReading
	for rows.Next() {
		var r domain.GlucoseReading
		if err := rows.Scan(&r.ID, &r.UserID, &r.ValueMmol, &r.Source, &r.RecordedAt); err != nil {
			return nil, fmt.Errorf("GlucoseRepo.GetLast: scan: %w", err)
		}
		readings = append(readings, r)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("GlucoseRepo.GetLast: rows: %w", err)
	}

	return readings, nil
}

func (r *GlucoseRepo) GetByTimeRange(ctx context.Context, userID int64, from, to time.Time) ([]domain.GlucoseReading, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, user_id, value_mmol, source, recorded_at
		 FROM glucose_readings
		 WHERE user_id = $1 AND recorded_at >= $2 AND recorded_at <= $3
		 ORDER BY recorded_at ASC`,
		userID, from, to,
	)
	if err != nil {
		return nil, fmt.Errorf("GlucoseRepo.GetByTimeRange: %w", err)
	}
	defer rows.Close()

	var readings []domain.GlucoseReading
	for rows.Next() {
		var rd domain.GlucoseReading
		if err := rows.Scan(&rd.ID, &rd.UserID, &rd.ValueMmol, &rd.Source, &rd.RecordedAt); err != nil {
			return nil, fmt.Errorf("GlucoseRepo.GetByTimeRange: scan: %w", err)
		}
		readings = append(readings, rd)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("GlucoseRepo.GetByTimeRange: rows: %w", err)
	}
	return readings, nil
}
