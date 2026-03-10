// Package glucose содержит бизнес-логику записи и просмотра уровня глюкозы.
package glucose

import (
	"context"
	"fmt"

	"github.com/gmtantsevov/shuggabuddy/internal/domain"
)

const (
	// MinMmol — минимальное допустимое значение в ммоль/л.
	MinMmol = 1.0
	// MaxMmol — максимальное допустимое значение в ммоль/л.
	MaxMmol = 33.3
	// MinMgdl — минимальное допустимое значение в мг/дл.
	MinMgdl = 18.0
	// MaxMgdl — максимальное допустимое значение в мг/дл.
	MaxMgdl = 600.0
)

type UseCase struct {
	repo domain.GlucoseRepository
}

func New(repo domain.GlucoseRepository) *UseCase {
	return &UseCase{repo: repo}
}

// SaveReading сохраняет измерение глюкозы.
// value интерпретируется в зависимости от units пользователя.
// В БД всегда сохраняется значение в ммоль/л.
func (uc *UseCase) SaveReading(ctx context.Context, userID int64, value float64, units domain.Units) error {
	var valueMmol float64

	switch units {
	case domain.UnitsMmol:
		if value < MinMmol || value > MaxMmol {
			return fmt.Errorf("glucose.SaveReading: value %.1f out of range [%.1f–%.1f] mmol/L", value, MinMmol, MaxMmol)
		}
		valueMmol = value
	case domain.UnitsMgdl:
		if value < MinMgdl || value > MaxMgdl {
			return fmt.Errorf("glucose.SaveReading: value %.0f out of range [%.0f–%.0f] mg/dL", value, MinMgdl, MaxMgdl)
		}
		valueMmol = value / domain.MmolToMgdl
	default:
		return fmt.Errorf("glucose.SaveReading: unknown units %q", units)
	}

	reading := &domain.GlucoseReading{
		UserID:    userID,
		ValueMmol: valueMmol,
		Source:    "manual",
	}

	if err := uc.repo.Save(ctx, reading); err != nil {
		return fmt.Errorf("glucose.SaveReading: %w", err)
	}

	return nil
}

// GetLastReadings возвращает последние записи пользователя.
func (uc *UseCase) GetLastReadings(ctx context.Context, userID int64, limit int) ([]domain.GlucoseReading, error) {
	readings, err := uc.repo.GetLast(ctx, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("glucose.GetLastReadings: %w", err)
	}

	return readings, nil
}

// ConvertToDisplay конвертирует значение из ммоль/л в указанные единицы.
func ConvertToDisplay(valueMmol float64, units domain.Units) float64 {
	if units == domain.UnitsMgdl {
		return valueMmol * domain.MmolToMgdl
	}
	return valueMmol
}

// FormatValue форматирует значение для отображения в зависимости от единиц.
func FormatValue(valueMmol float64, units domain.Units) string {
	if units == domain.UnitsMgdl {
		return fmt.Sprintf("%.0f", valueMmol*domain.MmolToMgdl)
	}
	return fmt.Sprintf("%.1f", valueMmol)
}
