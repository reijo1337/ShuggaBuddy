package bolus

import (
	"context"
	"fmt"
	"time"

	"github.com/gmtantsevov/shuggabuddy/internal/domain"
	"github.com/gmtantsevov/shuggabuddy/internal/usecase/insulincalc"
)

const (
	lookbackDays = 14
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

	lookbackFrom := now.Add(-lookbackDays * 24 * time.Hour)
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

	icr, err := insulincalc.DeriveICR(foods, doses, glucoseReadings, user.TargetMinMmol, user.TargetMaxMmol)
	if err != nil {
		return nil, err
	}

	isf, err := insulincalc.DeriveISF(foods, doses, glucoseReadings)
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
