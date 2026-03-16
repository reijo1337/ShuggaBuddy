package domain

import (
	"context"
	"time"
)

// GlucoseReading представляет единичное измерение уровня глюкозы.
type GlucoseReading struct {
	ID         int64     `json:"id"`
	UserID     int64     `json:"user_id"`
	ValueMmol  float64   `json:"value_mmol"`
	Source     string    `json:"source"`
	RecordedAt time.Time `json:"recorded_at"`
}

//go:generate mockgen -destination=mocks/mock_glucose_repository.go -package=mocks . GlucoseRepository

type GlucoseRepository interface {
	Save(ctx context.Context, reading *GlucoseReading) error
	GetLast(ctx context.Context, userID int64, limit int) ([]GlucoseReading, error)
	GetByTimeRange(ctx context.Context, userID int64, from, to time.Time) ([]GlucoseReading, error)
}
