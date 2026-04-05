package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gmtantsevov/shuggabuddy/internal/domain"
)

type InsulinRepo struct {
	pool *pgxpool.Pool
}

func NewInsulinRepo(pool *pgxpool.Pool) *InsulinRepo {
	return &InsulinRepo{pool: pool}
}

func (r *InsulinRepo) Save(ctx context.Context, dose *domain.InsulinDose) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO insulin_doses (user_id, dose_units, insulin_type, drug, source)
		 VALUES ($1, $2, $3, $4, $5)`,
		dose.UserID, dose.DoseUnits, dose.InsulinType, dose.Drug, dose.Source,
	)
	if err != nil {
		return fmt.Errorf("InsulinRepo.Save: %w", err)
	}
	return nil
}

func (r *InsulinRepo) GetLast(ctx context.Context, userID int64, limit int) ([]domain.InsulinDose, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, user_id, dose_units, insulin_type, drug, source, recorded_at
		 FROM insulin_doses
		 WHERE user_id = $1
		 ORDER BY recorded_at DESC
		 LIMIT $2`,
		userID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("InsulinRepo.GetLast: %w", err)
	}
	defer rows.Close()

	var doses []domain.InsulinDose
	for rows.Next() {
		var d domain.InsulinDose
		if err := rows.Scan(&d.ID, &d.UserID, &d.DoseUnits, &d.InsulinType, &d.Drug, &d.Source, &d.RecordedAt); err != nil {
			return nil, fmt.Errorf("InsulinRepo.GetLast: scan: %w", err)
		}
		doses = append(doses, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("InsulinRepo.GetLast: rows: %w", err)
	}
	return doses, nil
}

func (r *InsulinRepo) GetByTimeRange(ctx context.Context, userID int64, from, to time.Time) ([]*domain.InsulinDose, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, user_id, dose_units, insulin_type, drug, source, recorded_at
		 FROM insulin_doses
		 WHERE user_id = $1 AND recorded_at >= $2 AND recorded_at < $3
		 ORDER BY recorded_at DESC`,
		userID, from, to,
	)
	if err != nil {
		return nil, fmt.Errorf("InsulinRepo.GetByTimeRange: %w", err)
	}
	defer rows.Close()

	var doses []*domain.InsulinDose
	for rows.Next() {
		d := &domain.InsulinDose{}
		if err := rows.Scan(&d.ID, &d.UserID, &d.DoseUnits, &d.InsulinType, &d.Drug, &d.Source, &d.RecordedAt); err != nil {
			return nil, fmt.Errorf("InsulinRepo.GetByTimeRange: scan: %w", err)
		}
		doses = append(doses, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("InsulinRepo.GetByTimeRange: rows: %w", err)
	}
	return doses, nil
}
