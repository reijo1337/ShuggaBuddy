package domain

import (
	"context"
	"time"
)

// ActivityType определяет тип физической активности.
type ActivityType string

const (
	ActivityWalking   ActivityType = "walking"
	ActivityRunning   ActivityType = "running"
	ActivityCycling   ActivityType = "cycling"
	ActivityStrength  ActivityType = "strength"
	ActivitySwimming  ActivityType = "swimming"
	ActivityYoga      ActivityType = "yoga"
	ActivityDancing   ActivityType = "dancing"
	ActivityTeamSport ActivityType = "team_sport"
	ActivitySkiing    ActivityType = "skiing"
	ActivityOther     ActivityType = "other"
)

// RiskLevel описывает уровень гликемического риска после активности.
type RiskLevel string

const (
	RiskLow      RiskLevel = "low"
	RiskModerate RiskLevel = "moderate"
	RiskHigh     RiskLevel = "high"
)

// Intensity описывает интенсивность активности.
type Intensity string

const (
	IntensityLow    Intensity = "low"
	IntensityMedium Intensity = "medium"
	IntensityHigh   Intensity = "high"
)

// GlycemicImpact — оценка влияния активности на гликемию (не хранится в БД).
type GlycemicImpact struct {
	RiskLevel    RiskLevel
	MonitorHours int
}

// ActivityEntry представляет единичную запись физической активности.
type ActivityEntry struct {
	ID           int64        `json:"id"`
	UserID       int64        `json:"user_id"`
	ActivityType ActivityType `json:"activity_type"`
	CustomType   string       `json:"custom_type"`
	DurationMin  int          `json:"duration_min"`
	Intensity    Intensity    `json:"intensity"`
	RecordedAt   time.Time    `json:"recorded_at"`
	CreatedAt    time.Time    `json:"created_at"`
}

// ActivityAnalysis — результат корреляции активности с глюкозой.
type ActivityAnalysis struct {
	Entry      ActivityEntry
	GlucBefore *float64
	GlucAfter  *float64
	Delta      *float64
	TimeBefore *time.Time
	TimeAfter  *time.Time
}

//go:generate mockgen -destination=mocks/mock_activity_repository.go -package=mocks . ActivityRepository

// ActivityRepository описывает хранилище записей активности.
type ActivityRepository interface {
	Save(ctx context.Context, entry *ActivityEntry) (int64, error)
	GetByID(ctx context.Context, id int64) (*ActivityEntry, error)
	GetLast(ctx context.Context, userID int64, limit int) ([]ActivityEntry, error)
}
