package bolus

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/gmtantsevov/shuggabuddy/internal/domain"
)

const (
	MinChains               = 5
	LookbackDays            = 14
	MealBolusWindowMin      = 15
	CorrectionFoodWindowMin = 30
	PostBolusCheckMinH      = 2.0
	PostBolusCheckMaxH      = 4.0
)

// UseCase implements bolus recommendation logic.
type UseCase struct {
	userRepo    domain.UserRepository
	insulinRepo domain.InsulinRepository
	glucoseRepo domain.GlucoseRepository
	foodRepo    domain.FoodRepository
}

// New creates a new bolus UseCase.
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

// calculateIOB computes insulin-on-board using linear decay model.
func calculateIOB(doses []domain.InsulinDose, diaHours float64, now time.Time) float64 {
	var iob float64
	for _, d := range doses {
		if d.InsulinType != domain.InsulinTypeBolus {
			continue
		}
		elapsed := now.Sub(d.RecordedAt).Hours()
		if elapsed >= diaHours || elapsed < 0 {
			continue
		}
		remaining := d.DoseUnits * (1 - elapsed/diaHours)
		iob += remaining
	}
	return iob
}

// deriveICR derives insulin-to-carb ratio from historical meal+bolus+glucose chains.
// Only chains where the post-meal glucose fell within the target range are used.
func deriveICR(
	foods []domain.FoodEntry,
	doses []domain.InsulinDose,
	glucoseReadings []domain.GlucoseReading,
	targetMin, targetMax float64,
) (float64, error) {
	mealWindow := time.Duration(MealBolusWindowMin) * time.Minute
	postMin := time.Duration(PostBolusCheckMinH * float64(time.Hour))
	postMax := time.Duration(PostBolusCheckMaxH * float64(time.Hour))

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

	if len(ratios) < MinChains {
		return 0, fmt.Errorf("bolus.deriveICR: insufficient data (%d chains, need %d)", len(ratios), MinChains)
	}

	return median(ratios), nil
}

// deriveISF derives insulin sensitivity factor from correction-only boluses
// (boluses without nearby food entries).
func deriveISF(
	foods []domain.FoodEntry,
	doses []domain.InsulinDose,
	glucoseReadings []domain.GlucoseReading,
) (float64, error) {
	foodWindow := time.Duration(CorrectionFoodWindowMin) * time.Minute
	glucoseBeforeWindow := time.Duration(CorrectionFoodWindowMin) * time.Minute
	postMin := time.Duration(PostBolusCheckMinH * float64(time.Hour))
	postMax := time.Duration(PostBolusCheckMaxH * float64(time.Hour))

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

	if len(factors) < MinChains {
		return 0, fmt.Errorf("bolus.deriveISF: insufficient data (%d chains, need %d)", len(factors), MinChains)
	}

	return median(factors), nil
}

// median returns the median value of a float64 slice.
func median(values []float64) float64 {
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

// Calculate computes a bolus recommendation for a user given current glucose and planned carbs.
func (uc *UseCase) Calculate(ctx context.Context, userID int64, currentGlucose, carbsGrams float64, now time.Time) (*domain.BolusRecommendation, error) {
	user, err := uc.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("bolus.Calculate: %w", err)
	}
	if user == nil {
		return nil, fmt.Errorf("bolus.Calculate: user not found")
	}
	if user.BolusDrug == "" {
		return nil, fmt.Errorf("bolus.Calculate: bolus drug not set")
	}

	profile, ok := domain.BolusInsulinCatalog[user.BolusDrug]
	if !ok {
		return nil, fmt.Errorf("bolus.Calculate: unknown drug %q", user.BolusDrug)
	}

	lookbackFrom := now.Add(-LookbackDays * 24 * time.Hour)
	diaWindow := now.Add(-time.Duration(profile.DIA * float64(time.Hour)))

	insulinDoses, err := uc.insulinRepo.GetByTimeRange(ctx, userID, lookbackFrom, now)
	if err != nil {
		return nil, fmt.Errorf("bolus.Calculate: %w", err)
	}

	foodEntries, err := uc.foodRepo.GetByTimeRange(ctx, userID, lookbackFrom, now)
	if err != nil {
		return nil, fmt.Errorf("bolus.Calculate: %w", err)
	}

	glucoseReadings, err := uc.glucoseRepo.GetByTimeRange(ctx, userID, lookbackFrom, now)
	if err != nil {
		return nil, fmt.Errorf("bolus.Calculate: %w", err)
	}

	foods := make([]domain.FoodEntry, len(foodEntries))
	for i, f := range foodEntries {
		foods[i] = *f
	}

	doses := make([]domain.InsulinDose, len(insulinDoses))
	for i, d := range insulinDoses {
		doses[i] = *d
	}

	icr, err := deriveICR(foods, doses, glucoseReadings, user.TargetMinMmol, user.TargetMaxMmol)
	if err != nil {
		return nil, err
	}

	isf, err := deriveISF(foods, doses, glucoseReadings)
	if err != nil {
		return nil, err
	}

	var recentDoses []domain.InsulinDose
	for _, d := range doses {
		if d.RecordedAt.After(diaWindow) {
			recentDoses = append(recentDoses, d)
		}
	}
	iob := calculateIOB(recentDoses, profile.DIA, now)

	targetMid := (user.TargetMinMmol + user.TargetMaxMmol) / 2
	foodDose := carbsGrams / icr
	correctionDose := (currentGlucose - targetMid) / isf

	total := foodDose + correctionDose - iob
	if total < 0 {
		total = 0
	}

	return &domain.BolusRecommendation{
		FoodDose:       foodDose,
		CorrectionDose: correctionDose,
		IOB:            iob,
		TotalDose:      total,
		ICR:            icr,
		ISF:            isf,
		CurrentGlucose: currentGlucose,
		TargetGlucose:  targetMid,
		CarbsGrams:     carbsGrams,
	}, nil
}
