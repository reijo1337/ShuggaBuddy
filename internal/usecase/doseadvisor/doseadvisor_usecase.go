package doseadvisor

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/gmtantsevov/shuggabuddy/internal/domain"
	"github.com/gmtantsevov/shuggabuddy/internal/usecase/insulincalc"
)

const (
	lookbackDays     = 14
	minFastingCount  = 5
	fastingStartHour = 5
	fastingEndHour   = 9
	foodWindowHours  = 2
	largeDeviation   = 3.0
	trendThreshold   = 0.10
	halfPeriodDays   = 7
)

type UseCase struct {
	userRepo    domain.UserRepository
	insulinRepo domain.InsulinRepository
	glucoseRepo domain.GlucoseRepository
	foodRepo    domain.FoodRepository
}

func New(
	userRepo domain.UserRepository,
	insulinRepo domain.InsulinRepository,
	glucoseRepo domain.GlucoseRepository,
	foodRepo domain.FoodRepository,
) *UseCase {
	return &UseCase{
		userRepo:    userRepo,
		insulinRepo: insulinRepo,
		glucoseRepo: glucoseRepo,
		foodRepo:    foodRepo,
	}
}

// analyzeBasal analyzes fasting glucose trends and recommends basal dose adjustments.
func (uc *UseCase) analyzeBasal(ctx context.Context, user *domain.User, now time.Time) (*domain.BasalAdvice, error) {
	if user.BasalDose == 0 {
		return nil, nil
	}

	loc, err := time.LoadLocation(user.Timezone)
	if err != nil {
		loc = time.UTC
	}

	lookbackFrom := now.Add(-lookbackDays * 24 * time.Hour)

	readings, err := uc.glucoseRepo.GetByTimeRange(ctx, user.ID, lookbackFrom, now)
	if err != nil {
		return nil, fmt.Errorf("doseadvisor.analyzeBasal: %w", err)
	}

	foodEntries, err := uc.foodRepo.GetByTimeRange(ctx, user.ID, lookbackFrom, now)
	if err != nil {
		return nil, fmt.Errorf("doseadvisor.analyzeBasal: %w", err)
	}

	fasting := filterFasting(readings, foodEntries, loc)
	if len(fasting) < minFastingCount {
		return nil, nil
	}

	avg := avgGlucose(fasting)

	advice := &domain.BasalAdvice{
		FastingAvg:   avg,
		FastingCount: len(fasting),
		CurrentDose:  user.BasalDose,
		TargetMin:    user.TargetMinMmol,
		TargetMax:    user.TargetMaxMmol,
	}

	targetMid := (user.TargetMinMmol + user.TargetMaxMmol) / 2

	switch {
	case avg > user.TargetMaxMmol:
		advice.Trend = domain.TrendHigh
		deviation := avg - targetMid
		step := 1.0
		if deviation > largeDeviation {
			step = 2.0
		}
		advice.SuggestedDose = user.BasalDose + step
	case avg < user.TargetMinMmol:
		advice.Trend = domain.TrendLow
		deviation := targetMid - avg
		step := 1.0
		if deviation > largeDeviation {
			step = 2.0
		}
		advice.SuggestedDose = math.Max(0.5, user.BasalDose-step)
	default:
		advice.Trend = domain.TrendStable
		advice.SuggestedDose = user.BasalDose
	}

	return advice, nil
}

// analyzeBolus analyzes ICR/ISF trends by comparing two 7-day periods.
func (uc *UseCase) analyzeBolus(ctx context.Context, user *domain.User, now time.Time) (*domain.BolusAdvice, error) {
	lookbackFrom := now.Add(-lookbackDays * 24 * time.Hour)
	midpoint := now.Add(-halfPeriodDays * 24 * time.Hour)

	foodEntries, err := uc.foodRepo.GetByTimeRange(ctx, user.ID, lookbackFrom, now)
	if err != nil {
		return nil, fmt.Errorf("doseadvisor.analyzeBolus: %w", err)
	}

	insulinDoses, err := uc.insulinRepo.GetByTimeRange(ctx, user.ID, lookbackFrom, now)
	if err != nil {
		return nil, fmt.Errorf("doseadvisor.analyzeBolus: %w", err)
	}

	glucoseReadings, err := uc.glucoseRepo.GetByTimeRange(ctx, user.ID, lookbackFrom, now)
	if err != nil {
		return nil, fmt.Errorf("doseadvisor.analyzeBolus: %w", err)
	}

	foodsPrev, foodsCurr := splitFoods(foodEntries, midpoint)
	dosesPrev, dosesCurr := splitDoses(insulinDoses, midpoint)
	glucPrev, glucCurr := splitGlucose(glucoseReadings, midpoint)

	prevICR, errICR1 := insulincalc.DeriveICR(foodsPrev, dosesPrev, glucPrev, user.TargetMinMmol, user.TargetMaxMmol)
	currICR, errICR2 := insulincalc.DeriveICR(foodsCurr, dosesCurr, glucCurr, user.TargetMinMmol, user.TargetMaxMmol)

	prevISF, errISF1 := insulincalc.DeriveISF(foodsPrev, dosesPrev, glucPrev)
	currISF, errISF2 := insulincalc.DeriveISF(foodsCurr, dosesCurr, glucCurr)

	if errICR1 != nil || errICR2 != nil {
		// Insufficient data to derive ICR — no bolus advice, not an error.
		return nil, nil //nolint:nilerr // insufficient data is not an error
	}

	advice := &domain.BolusAdvice{
		CurrentICR:  currICR,
		PreviousICR: prevICR,
		TargetMin:   user.TargetMinMmol,
		TargetMax:   user.TargetMaxMmol,
	}

	icrChange := (currICR - prevICR) / prevICR
	switch {
	case icrChange < -trendThreshold:
		advice.ICRTrend = domain.TrendHigh
	case icrChange > trendThreshold:
		advice.ICRTrend = domain.TrendLow
	default:
		advice.ICRTrend = domain.TrendStable
	}

	if errISF1 == nil && errISF2 == nil {
		advice.CurrentISF = currISF
		advice.PreviousISF = prevISF
		isfChange := (currISF - prevISF) / prevISF
		switch {
		case isfChange < -trendThreshold:
			advice.ISFTrend = domain.TrendHigh
		case isfChange > trendThreshold:
			advice.ISFTrend = domain.TrendLow
		default:
			advice.ISFTrend = domain.TrendStable
		}
	} else {
		advice.ISFTrend = domain.TrendStable
	}

	postMealReadings := filterPostMeal(glucoseReadings, foodEntries)
	if len(postMealReadings) > 0 {
		advice.PostMealAvg = avgGlucose(postMealReadings)
		advice.PostMealCount = len(postMealReadings)
	}

	return advice, nil
}

// Analyze runs both basal and bolus analysis.
func (uc *UseCase) Analyze(ctx context.Context, userID int64, now time.Time) (*domain.DoseAdvice, error) {
	user, err := uc.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("doseadvisor.Analyze: %w", err)
	}
	if user == nil {
		return nil, fmt.Errorf("doseadvisor.Analyze: user not found")
	}

	advice := &domain.DoseAdvice{AnalyzedAt: now}

	basalAdvice, err := uc.analyzeBasal(ctx, user, now)
	if err != nil {
		return nil, fmt.Errorf("doseadvisor.Analyze: %w", err)
	}
	advice.Basal = basalAdvice

	bolusAdvice, err := uc.analyzeBolus(ctx, user, now)
	if err != nil {
		return nil, fmt.Errorf("doseadvisor.Analyze: %w", err)
	}
	advice.Bolus = bolusAdvice

	return advice, nil
}

func filterFasting(readings []domain.GlucoseReading, foods []*domain.FoodEntry, loc *time.Location) []domain.GlucoseReading {
	foodWindow := time.Duration(foodWindowHours) * time.Hour

	var result []domain.GlucoseReading
	for _, r := range readings {
		localTime := r.RecordedAt.In(loc)
		hour := localTime.Hour()
		if hour < fastingStartHour || hour >= fastingEndHour {
			continue
		}

		hasFood := false
		for _, f := range foods {
			gap := r.RecordedAt.Sub(f.EatenAt)
			if gap < 0 {
				gap = -gap
			}
			if gap <= foodWindow {
				hasFood = true
				break
			}
		}
		if hasFood {
			continue
		}

		result = append(result, r)
	}
	return result
}

func avgGlucose(readings []domain.GlucoseReading) float64 {
	if len(readings) == 0 {
		return 0
	}
	var sum float64
	for _, r := range readings {
		sum += r.ValueMmol
	}
	return sum / float64(len(readings))
}

func splitFoods(entries []*domain.FoodEntry, mid time.Time) (prev, curr []domain.FoodEntry) {
	for _, e := range entries {
		if e.EatenAt.Before(mid) {
			prev = append(prev, *e)
		} else {
			curr = append(curr, *e)
		}
	}
	return prev, curr
}

func splitDoses(entries []*domain.InsulinDose, mid time.Time) (prev, curr []domain.InsulinDose) {
	for _, e := range entries {
		if e.RecordedAt.Before(mid) {
			prev = append(prev, *e)
		} else {
			curr = append(curr, *e)
		}
	}
	return prev, curr
}

func splitGlucose(entries []domain.GlucoseReading, mid time.Time) (prev, curr []domain.GlucoseReading) {
	for _, e := range entries {
		if e.RecordedAt.Before(mid) {
			prev = append(prev, e)
		} else {
			curr = append(curr, e)
		}
	}
	return prev, curr
}

func filterPostMeal(readings []domain.GlucoseReading, foods []*domain.FoodEntry) []domain.GlucoseReading {
	postMin := 2 * time.Hour
	postMax := 4 * time.Hour

	var result []domain.GlucoseReading
	for _, r := range readings {
		for _, f := range foods {
			elapsed := r.RecordedAt.Sub(f.EatenAt)
			if elapsed >= postMin && elapsed <= postMax {
				result = append(result, r)
				break
			}
		}
	}
	return result
}
