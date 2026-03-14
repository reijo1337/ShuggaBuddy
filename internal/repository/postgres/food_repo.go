package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gmtantsevov/shuggabuddy/internal/domain"
)

type FoodRepo struct {
	pool *pgxpool.Pool
}

func NewFoodRepo(pool *pgxpool.Pool) *FoodRepo {
	return &FoodRepo{pool: pool}
}

func (r *FoodRepo) Save(ctx context.Context, entry *domain.FoodEntry) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO food_entries (user_id, carbs_grams, note, eaten_at)
		 VALUES ($1, $2, $3, $4)`,
		entry.UserID, entry.CarbsGrams, entry.Note, entry.EatenAt,
	)
	if err != nil {
		return fmt.Errorf("FoodRepo.Save: %w", err)
	}
	return nil
}

func (r *FoodRepo) GetLast(ctx context.Context, userID int64, limit int) ([]domain.FoodEntry, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, user_id, carbs_grams, note, eaten_at, created_at
		 FROM food_entries
		 WHERE user_id = $1
		 ORDER BY eaten_at DESC
		 LIMIT $2`,
		userID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("FoodRepo.GetLast: %w", err)
	}
	defer rows.Close()

	var entries []domain.FoodEntry
	for rows.Next() {
		var e domain.FoodEntry
		if err := rows.Scan(&e.ID, &e.UserID, &e.CarbsGrams, &e.Note, &e.EatenAt, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("FoodRepo.GetLast: scan: %w", err)
		}
		entries = append(entries, e)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("FoodRepo.GetLast: rows: %w", err)
	}

	return entries, nil
}
