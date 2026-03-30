// Package postgres содержит реализации репозиториев через pgx/v5.
package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gmtantsevov/shuggabuddy/internal/domain"
)

type UserRepo struct {
	pool *pgxpool.Pool
}

func NewUserRepo(pool *pgxpool.Pool) *UserRepo {
	return &UserRepo{pool: pool}
}

func (r *UserRepo) GetByID(ctx context.Context, id int64) (*domain.User, error) {
	u := &domain.User{}
	err := r.pool.QueryRow(ctx,
		`SELECT id, units, carbs_per_unit, created_at, target_min_mmol, target_max_mmol, basal_drug, basal_time, timezone FROM users WHERE id = $1`, id,
	).Scan(&u.ID, &u.Units, &u.CarbsPerUnit, &u.CreatedAt, &u.TargetMinMmol, &u.TargetMaxMmol, &u.BasalDrug, &u.BasalTime, &u.Timezone)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("UserRepo.GetByID: %w", err)
	}

	return u, nil
}

func (r *UserRepo) Create(ctx context.Context, user *domain.User) error {
	err := r.pool.QueryRow(ctx,
		`INSERT INTO users (units, carbs_per_unit) VALUES ($1, $2) RETURNING id`,
		user.Units, user.CarbsPerUnit,
	).Scan(&user.ID)
	if err != nil {
		return fmt.Errorf("UserRepo.Create: %w", err)
	}
	return nil
}

func (r *UserRepo) UpdateUnits(ctx context.Context, id int64, units domain.Units) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE users SET units = $1 WHERE id = $2`,
		units, id,
	)
	if err != nil {
		return fmt.Errorf("UserRepo.UpdateUnits: %w", err)
	}

	return nil
}

func (r *UserRepo) UpdateCarbsPerUnit(ctx context.Context, id int64, grams float64) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE users SET carbs_per_unit = $1 WHERE id = $2`,
		grams, id,
	)
	if err != nil {
		return fmt.Errorf("UserRepo.UpdateCarbsPerUnit: %w", err)
	}
	return nil
}

func (r *UserRepo) UpdateTimezone(ctx context.Context, userID int64, timezone string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE users SET timezone = $1 WHERE id = $2`,
		timezone, userID,
	)
	if err != nil {
		return fmt.Errorf("UserRepo.UpdateTimezone: %w", err)
	}
	return nil
}

func (r *UserRepo) UpdateSettings(ctx context.Context, userID int64, targetMin, targetMax float64, basalDrug, basalTime string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE users SET target_min_mmol = $1, target_max_mmol = $2, basal_drug = $3, basal_time = $4 WHERE id = $5`,
		targetMin, targetMax, basalDrug, basalTime, userID,
	)
	if err != nil {
		return fmt.Errorf("UserRepo.UpdateSettings: %w", err)
	}
	return nil
}
