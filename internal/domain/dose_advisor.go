package domain

import "time"

// TrendDirection describes the direction of a glucose trend.
type TrendDirection string

const (
	TrendHigh   TrendDirection = "high"
	TrendLow    TrendDirection = "low"
	TrendStable TrendDirection = "stable"
)

// BasalAdvice holds the result of basal insulin trend analysis.
type BasalAdvice struct {
	Trend         TrendDirection
	FastingAvg    float64
	FastingCount  int
	CurrentDose   float64
	SuggestedDose float64
	TargetMin     float64
	TargetMax     float64
}

// BolusAdvice holds the result of bolus insulin trend analysis.
type BolusAdvice struct {
	ICRTrend      TrendDirection
	ISFTrend      TrendDirection
	CurrentICR    float64
	CurrentISF    float64
	PreviousICR   float64
	PreviousISF   float64
	PostMealAvg   float64
	PostMealCount int
	TargetMin     float64
	TargetMax     float64
}

// DoseAdvice is the combined recommendation.
type DoseAdvice struct {
	Basal      *BasalAdvice
	Bolus      *BolusAdvice
	AnalyzedAt time.Time
}
