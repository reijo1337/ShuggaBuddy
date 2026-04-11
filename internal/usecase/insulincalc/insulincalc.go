package insulincalc

import (
	"fmt"
	"sort"
	"time"

	"github.com/gmtantsevov/shuggabuddy/internal/domain"
)

const (
	minChains               = 5
	mealBolusWindowMin      = 15
	correctionFoodWindowMin = 30
	postBolusCheckMinH      = 2.0
	postBolusCheckMaxH      = 4.0
)

// DeriveICR derives insulin-to-carb ratio from historical meal+bolus+glucose chains.
// Only chains where the post-meal glucose fell within the target range are used.
func DeriveICR(
	foods []domain.FoodEntry,
	doses []domain.InsulinDose,
	glucoseReadings []domain.GlucoseReading,
	targetMin, targetMax float64,
) (float64, error) {
	mealWindow := time.Duration(mealBolusWindowMin) * time.Minute
	postMin := time.Duration(postBolusCheckMinH * float64(time.Hour))
	postMax := time.Duration(postBolusCheckMaxH * float64(time.Hour))

	var ratios []float64

	for _, food := range foods {
		var matchedDose *domain.InsulinDose
		var bestGap time.Duration
		for i := range doses {
			if doses[i].InsulinType != domain.InsulinTypeBolus {
				continue
			}
			gap := doses[i].RecordedAt.Sub(food.EatenAt)
			if gap < 0 {
				gap = -gap
			}
			if gap <= mealWindow && (matchedDose == nil || gap < bestGap) {
				matchedDose = &doses[i]
				bestGap = gap
			}
		}
		if matchedDose == nil || matchedDose.DoseUnits <= 0 || food.CarbsGrams <= 0 {
			continue
		}

		targetElapsed := time.Duration(3 * float64(time.Hour))
		var postGlucose *domain.GlucoseReading
		var bestDist time.Duration
		for i := range glucoseReadings {
			elapsed := glucoseReadings[i].RecordedAt.Sub(matchedDose.RecordedAt)
			if elapsed >= postMin && elapsed <= postMax {
				dist := elapsed - targetElapsed
				if dist < 0 {
					dist = -dist
				}
				if postGlucose == nil || dist < bestDist {
					postGlucose = &glucoseReadings[i]
					bestDist = dist
				}
			}
		}
		if postGlucose == nil {
			continue
		}

		if postGlucose.ValueMmol < targetMin || postGlucose.ValueMmol > targetMax {
			continue
		}

		ratios = append(ratios, food.CarbsGrams/matchedDose.DoseUnits)
	}

	if len(ratios) < minChains {
		return 0, fmt.Errorf("insulincalc.DeriveICR: insufficient data (%d chains, need %d)", len(ratios), minChains)
	}

	return Median(ratios), nil
}

// DeriveISF derives insulin sensitivity factor from correction-only boluses
// (boluses without nearby food entries).
func DeriveISF(
	foods []domain.FoodEntry,
	doses []domain.InsulinDose,
	glucoseReadings []domain.GlucoseReading,
) (float64, error) {
	foodWindow := time.Duration(correctionFoodWindowMin) * time.Minute
	glucoseBeforeWindow := time.Duration(correctionFoodWindowMin) * time.Minute
	postMin := time.Duration(postBolusCheckMinH * float64(time.Hour))
	postMax := time.Duration(postBolusCheckMaxH * float64(time.Hour))

	var factors []float64

	for _, dose := range doses {
		if dose.InsulinType != domain.InsulinTypeBolus || dose.DoseUnits <= 0 {
			continue
		}

		hasFood := false
		for _, f := range foods {
			gap := dose.RecordedAt.Sub(f.EatenAt)
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

		var glucoseBefore *domain.GlucoseReading
		for i := range glucoseReadings {
			diff := dose.RecordedAt.Sub(glucoseReadings[i].RecordedAt)
			if diff >= 0 && diff <= glucoseBeforeWindow {
				if glucoseBefore == nil || diff < dose.RecordedAt.Sub(glucoseBefore.RecordedAt) {
					glucoseBefore = &glucoseReadings[i]
				}
			}
		}
		if glucoseBefore == nil {
			continue
		}

		// Select reading closest to 3h post-bolus for more consistent ISF.
		targetElapsed := time.Duration(3 * float64(time.Hour))
		var glucoseAfter *domain.GlucoseReading
		var bestDist time.Duration
		for i := range glucoseReadings {
			elapsed := glucoseReadings[i].RecordedAt.Sub(dose.RecordedAt)
			if elapsed >= postMin && elapsed <= postMax {
				dist := elapsed - targetElapsed
				if dist < 0 {
					dist = -dist
				}
				if glucoseAfter == nil || dist < bestDist {
					glucoseAfter = &glucoseReadings[i]
					bestDist = dist
				}
			}
		}
		if glucoseAfter == nil {
			continue
		}

		drop := glucoseBefore.ValueMmol - glucoseAfter.ValueMmol
		if drop <= 0 {
			continue
		}

		factors = append(factors, drop/dose.DoseUnits)
	}

	if len(factors) < minChains {
		return 0, fmt.Errorf("insulincalc.DeriveISF: insufficient data (%d chains, need %d)", len(factors), minChains)
	}

	return Median(factors), nil
}

// Median returns the median value of a float64 slice.
func Median(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sort.Float64s(values)
	n := len(values)
	if n%2 == 0 {
		return (values[n/2-1] + values[n/2]) / 2
	}
	return values[n/2]
}
