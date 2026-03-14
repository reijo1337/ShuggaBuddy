package domain

import (
	"context"
	"time"
)

// FoodEntry represents a single meal/carbohydrate intake record.
type FoodEntry struct {
	ID         int64     `json:"id"`
	UserID     int64     `json:"user_id"`
	CarbsGrams float64   `json:"carbs_grams"` // always stored in grams
	Note       string    `json:"note"`        // optional
	EatenAt    time.Time `json:"eaten_at"`    // meal time
	CreatedAt  time.Time `json:"created_at"`
}

//go:generate mockgen -destination=mocks/mock_food_repository.go -package=mocks . FoodRepository

type FoodRepository interface {
	Save(ctx context.Context, entry *FoodEntry) error
	GetLast(ctx context.Context, userID int64, limit int) ([]FoodEntry, error)
}
