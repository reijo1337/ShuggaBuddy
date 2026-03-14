// Package food содержит бизнес-логику записи приёма пищи.
package food

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/gmtantsevov/shuggabuddy/internal/domain"
)

const (
	MinCarbsGrams = 0.1
	MaxCarbsGrams = 500.0
)

type UseCase struct {
	repo domain.FoodRepository
}

func New(repo domain.FoodRepository) *UseCase {
	return &UseCase{repo: repo}
}

// SaveEntry saves a food entry. carbsGrams must be in [0.1, 500].
func (uc *UseCase) SaveEntry(ctx context.Context, userID int64, carbsGrams float64, note string, eatenAt time.Time) error {
	if carbsGrams < MinCarbsGrams || carbsGrams > MaxCarbsGrams {
		return fmt.Errorf("food.SaveEntry: carbsGrams %.1f out of range [%.1f–%.1f]",
			carbsGrams, MinCarbsGrams, MaxCarbsGrams)
	}

	entry := &domain.FoodEntry{
		UserID:     userID,
		CarbsGrams: carbsGrams,
		Note:       note,
		EatenAt:    eatenAt,
	}

	if err := uc.repo.Save(ctx, entry); err != nil {
		return fmt.Errorf("food.SaveEntry: %w", err)
	}

	return nil
}

// GetLastEntries returns the most recent food entries for a user.
func (uc *UseCase) GetLastEntries(ctx context.Context, userID int64, limit int) ([]domain.FoodEntry, error) {
	entries, err := uc.repo.GetLast(ctx, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("food.GetLastEntries: %w", err)
	}
	return entries, nil
}

// FormatCarbs formats carbs grams for display, stripping trailing zeros.
// Examples: 60.0 → "60", 47.5 → "47.5"
func FormatCarbs(grams float64) string {
	return strconv.FormatFloat(grams, 'f', -1, 64)
}
