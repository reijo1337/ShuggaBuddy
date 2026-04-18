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
		`INSERT INTO glucose_readings (user_id, value_mmol, source, trend)
		 VALUES ($1, $2, $3, $4)`,
		reading.UserID, reading.ValueMmol, reading.Source, reading.Trend,
	)
	if err != nil {
		return fmt.Errorf("GlucoseRepo.Save: %w", err)
	}

	return nil
}

func (r *GlucoseRepo) SaveBatch(ctx context.Context, readings []domain.GlucoseReading) (int, error) {
	if len(readings) == 0 {
		return 0, nil
	}

	inserted := 0
	for _, rd := range readings {
		tag, err := r.pool.Exec(ctx,
			`INSERT INTO glucose_readings (user_id, value_mmol, source, trend, recorded_at)
			 VALUES ($1, $2, $3, $4, $5)
			 ON CONFLICT DO NOTHING`,
			rd.UserID, rd.ValueMmol, rd.Source, rd.Trend, rd.RecordedAt,
		)
		if err != nil {
			return inserted, fmt.Errorf("GlucoseRepo.SaveBatch: %w", err)
		}
		inserted += int(tag.RowsAffected())
	}

	return inserted, nil
}

func (r *GlucoseRepo) GetLast(ctx context.Context, userID int64, limit int) ([]domain.GlucoseReading, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, user_id, value_mmol, source, trend, recorded_at
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
		if err := rows.Scan(&r.ID, &r.UserID, &r.ValueMmol, &r.Source, &r.Trend, &r.RecordedAt); err != nil {
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
		`SELECT id, user_id, value_mmol, source, trend, recorded_at
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
		if err := rows.Scan(&rd.ID, &rd.UserID, &rd.ValueMmol, &rd.Source, &rd.Trend, &rd.RecordedAt); err != nil {
			return nil, fmt.Errorf("GlucoseRepo.GetByTimeRange: scan: %w", err)
		}
		readings = append(readings, rd)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("GlucoseRepo.GetByTimeRange: rows: %w", err)
	}
	return readings, nil
}
