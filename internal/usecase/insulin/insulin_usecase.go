// Package insulin содержит бизнес-логику записи и просмотра доз инсулина.
package insulin

import (
	"context"
	"fmt"

	"github.com/gmtantsevov/shuggabuddy/internal/domain"
)

const (
	// MaxDoseUnits — жёсткий максимум дозы для обоих типов.
	MaxDoseUnits = 200.0
	// AnomalyBolusUnits — порог аномальной болюсной дозы (предупреждение на UI).
	AnomalyBolusUnits = 50.0
	// AnomalyBasalUnits — порог аномальной базальной дозы (предупреждение на UI).
	AnomalyBasalUnits = 100.0
)

// UseCase содержит бизнес-логику записи инсулина.
type UseCase struct {
	repo domain.InsulinRepository
}

// New создаёт новый UseCase.
func New(repo domain.InsulinRepository) *UseCase {
	return &UseCase{repo: repo}
}

// SaveDose сохраняет дозу инсулина.
// dose должна быть > 0 и <= MaxDoseUnits.
func (uc *UseCase) SaveDose(ctx context.Context, userID int64, dose float64, insulinType domain.InsulinType, drug string) error {
	if dose <= 0 {
		return fmt.Errorf("insulin.SaveDose: dose must be positive, got %.2f", dose)
	}
	if dose > MaxDoseUnits {
		return fmt.Errorf("insulin.SaveDose: dose %.2f out of range (max %.0f units)", dose, MaxDoseUnits)
	}

	d := &domain.InsulinDose{
		UserID:      userID,
		DoseUnits:   dose,
		InsulinType: insulinType,
		Drug:        drug,
	}

	if err := uc.repo.Save(ctx, d); err != nil {
		return fmt.Errorf("insulin.SaveDose: %w", err)
	}
	return nil
}

// GetLastDoses возвращает последние записи пользователя.
func (uc *UseCase) GetLastDoses(ctx context.Context, userID int64, limit int) ([]domain.InsulinDose, error) {
	doses, err := uc.repo.GetLast(ctx, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("insulin.GetLastDoses: %w", err)
	}
	return doses, nil
}

// IsAnomalousDose проверяет, превышает ли доза порог предупреждения.
// Используется delivery-слоем для отображения предупреждения пользователю.
func IsAnomalousDose(dose float64, insulinType domain.InsulinType) bool {
	switch insulinType {
	case domain.InsulinTypeBolus:
		return dose > AnomalyBolusUnits
	case domain.InsulinTypeBasal:
		return dose > AnomalyBasalUnits
	default:
		return false
	}
}
