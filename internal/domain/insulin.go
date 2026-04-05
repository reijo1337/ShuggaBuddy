package domain

import (
	"context"
	"time"
)

// InsulinType определяет тип инсулина.
type InsulinType string

const (
	InsulinTypeBolus InsulinType = "bolus" // быстрый
	InsulinTypeBasal InsulinType = "basal" // длинный/базовый
)

// InsulinDose представляет единичную запись дозы инсулина.
type InsulinDose struct {
	ID          int64       `json:"id"`
	UserID      int64       `json:"user_id"`
	DoseUnits   float64     `json:"dose_units"`
	InsulinType InsulinType `json:"insulin_type"`
	Drug        string      `json:"drug"`   // пустая строка = не указан
	Source      string      `json:"source"` // "manual" | "bolus_calculator"
	RecordedAt  time.Time   `json:"recorded_at"`
}

//go:generate mockgen -destination=mocks/mock_insulin_repository.go -package=mocks . InsulinRepository

// InsulinRepository описывает хранилище доз инсулина.
type InsulinRepository interface {
	Save(ctx context.Context, dose *InsulinDose) error
	GetLast(ctx context.Context, userID int64, limit int) ([]InsulinDose, error)
	GetByTimeRange(ctx context.Context, userID int64, from, to time.Time) ([]*InsulinDose, error)
}
