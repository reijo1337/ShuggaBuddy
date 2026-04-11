// Package postgres содержит реализации репозиториев через pgx/v5.
package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

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
		`SELECT id, units, carbs_per_unit, created_at, target_min_mmol, target_max_mmol, basal_drug, basal_time, bolus_drug, timezone, basal_dose, advisor_interval_days, advisor_last_sent_at FROM users WHERE id = $1`, id,
	).Scan(&u.ID, &u.Units, &u.CarbsPerUnit, &u.CreatedAt, &u.TargetMinMmol, &u.TargetMaxMmol, &u.BasalDrug, &u.BasalTime, &u.BolusDrug, &u.Timezone, &u.BasalDose, &u.AdvisorIntervalDays, &u.AdvisorLastSentAt)

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

func (r *UserRepo) UpdateBolusDrug(ctx context.Context, userID int64, drug string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE users SET bolus_drug = $1 WHERE id = $2`,
		drug, userID,
	)
	if err != nil {
		return fmt.Errorf("UserRepo.UpdateBolusDrug: %w", err)
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

func (r *UserRepo) UpdateBasalDose(ctx context.Context, userID int64, dose float64) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE users SET basal_dose = $1 WHERE id = $2`,
		dose, userID,
	)
	if err != nil {
		return fmt.Errorf("UserRepo.UpdateBasalDose: %w", err)
	}
	return nil
}

func (r *UserRepo) UpdateAdvisorInterval(ctx context.Context, userID int64, days int) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE users SET advisor_interval_days = $1 WHERE id = $2`,
		days, userID,
	)
	if err != nil {
		return fmt.Errorf("UserRepo.UpdateAdvisorInterval: %w", err)
	}
	return nil
}

func (r *UserRepo) UpdateAdvisorLastSentAt(ctx context.Context, userID int64, sentAt time.Time) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE users SET advisor_last_sent_at = $1 WHERE id = $2`,
		sentAt, userID,
	)
	if err != nil {
		return fmt.Errorf("UserRepo.UpdateAdvisorLastSentAt: %w", err)
	}
	return nil
}

func (r *UserRepo) GetUsersForAdvisor(ctx context.Context, now time.Time) ([]domain.User, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, units, carbs_per_unit, created_at, target_min_mmol, target_max_mmol,
		        basal_drug, basal_time, bolus_drug, timezone,
		        basal_dose, advisor_interval_days, advisor_last_sent_at
		 FROM users
		 WHERE advisor_interval_days > 0
		   AND (advisor_last_sent_at IS NULL
		        OR advisor_last_sent_at + (advisor_interval_days * INTERVAL '1 day') <= $1)`,
		now,
	)
	if err != nil {
		return nil, fmt.Errorf("UserRepo.GetUsersForAdvisor: %w", err)
	}
	defer rows.Close()

	var users []domain.User
	for rows.Next() {
		var u domain.User
		if err := rows.Scan(
			&u.ID, &u.Units, &u.CarbsPerUnit, &u.CreatedAt,
			&u.TargetMinMmol, &u.TargetMaxMmol,
			&u.BasalDrug, &u.BasalTime, &u.BolusDrug, &u.Timezone,
			&u.BasalDose, &u.AdvisorIntervalDays, &u.AdvisorLastSentAt,
		); err != nil {
			return nil, fmt.Errorf("UserRepo.GetUsersForAdvisor scan: %w", err)
		}
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("UserRepo.GetUsersForAdvisor rows: %w", err)
	}
	return users, nil
}
