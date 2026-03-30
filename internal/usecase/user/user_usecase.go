// Package user содержит бизнес-логику управления профилем пользователя.
package user

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gmtantsevov/shuggabuddy/internal/domain"
)

type UseCase struct {
	userRepo   domain.UserRepository
	extAccRepo domain.ExternalAccountRepository
}

func New(userRepo domain.UserRepository, extAccRepo domain.ExternalAccountRepository) *UseCase {
	return &UseCase{userRepo: userRepo, extAccRepo: extAccRepo}
}

func (uc *UseCase) GetOrCreateUser(ctx context.Context, provider domain.Provider, externalID, displayName string) (*domain.User, *domain.ExternalAccount, bool, error) {
	acc, err := uc.extAccRepo.GetByProvider(ctx, provider, externalID)
	if err != nil {
		return nil, nil, false, fmt.Errorf("user.GetOrCreateUser: %w", err)
	}

	if acc != nil {
		u, err := uc.userRepo.GetByID(ctx, acc.UserID)
		if err != nil {
			return nil, nil, false, fmt.Errorf("user.GetOrCreateUser: %w", err)
		}
		return u, acc, false, nil
	}

	u := &domain.User{Units: domain.UnitsMmol, CarbsPerUnit: 12}
	if err := uc.userRepo.Create(ctx, u); err != nil {
		return nil, nil, false, fmt.Errorf("user.GetOrCreateUser: %w", err)
	}

	acc = &domain.ExternalAccount{
		UserID:      u.ID,
		Provider:    provider,
		ExternalID:  externalID,
		DisplayName: displayName,
	}
	if err := uc.extAccRepo.Create(ctx, acc); err != nil {
		return nil, nil, false, fmt.Errorf("user.GetOrCreateUser: %w", err)
	}

	return u, acc, true, nil
}

func (uc *UseCase) GetProfile(ctx context.Context, provider domain.Provider, externalID string) (*domain.User, *domain.ExternalAccount, error) {
	acc, err := uc.extAccRepo.GetByProvider(ctx, provider, externalID)
	if err != nil {
		return nil, nil, fmt.Errorf("user.GetProfile: %w", err)
	}

	if acc == nil {
		return nil, nil, nil
	}

	u, err := uc.userRepo.GetByID(ctx, acc.UserID)
	if err != nil {
		return nil, nil, fmt.Errorf("user.GetProfile: %w", err)
	}

	return u, acc, nil
}

func (uc *UseCase) UpdateUnits(ctx context.Context, id int64, units domain.Units) error {
	if units != domain.UnitsMmol && units != domain.UnitsMgdl {
		return fmt.Errorf("user.UpdateUnits: invalid units %q", units)
	}

	if err := uc.userRepo.UpdateUnits(ctx, id, units); err != nil {
		return fmt.Errorf("user.UpdateUnits: %w", err)
	}

	return nil
}

// UpdateSettings updates the user's target glucose range and basal insulin settings.
// targetMin and targetMax must be in [1.0, 33.3] mmol/L, and targetMax must be greater than targetMin.
// basalTime must be either empty or in "HH:MM" format (24h).
func (uc *UseCase) UpdateSettings(ctx context.Context, userID int64, targetMin, targetMax float64, basalDrug, basalTime string) error {
	const minGlucose, maxGlucose = 1.0, 33.3

	if targetMin < minGlucose || targetMin > maxGlucose {
		return fmt.Errorf("user.UpdateSettings: targetMin %.1f out of range [1.0–33.3]", targetMin)
	}
	if targetMax < minGlucose || targetMax > maxGlucose {
		return fmt.Errorf("user.UpdateSettings: targetMax %.1f out of range [1.0–33.3]", targetMax)
	}
	if targetMax <= targetMin {
		return fmt.Errorf("user.UpdateSettings: targetMax must be greater than targetMin")
	}

	if basalTime != "" {
		parts := strings.Split(basalTime, ":")
		if len(parts) != 2 {
			return fmt.Errorf("user.UpdateSettings: invalid basalTime %q, expected HH:MM", basalTime)
		}
		h, errH := strconv.Atoi(parts[0])
		m, errM := strconv.Atoi(parts[1])
		if errH != nil || errM != nil || h < 0 || h > 23 || m < 0 || m > 59 {
			return fmt.Errorf("user.UpdateSettings: invalid basalTime %q, expected HH:MM", basalTime)
		}
	}

	return uc.userRepo.UpdateSettings(ctx, userID, targetMin, targetMax, basalDrug, basalTime)
}

// UpdateTimezone updates the user's IANA timezone (e.g. "Europe/Moscow").
// Returns an error if the timezone name is not recognised by Go's time package.
func (uc *UseCase) UpdateTimezone(ctx context.Context, userID int64, timezone string) error {
	if _, err := time.LoadLocation(timezone); err != nil {
		return fmt.Errorf("user.UpdateTimezone: unknown timezone %q: %w", timezone, err)
	}
	return uc.userRepo.UpdateTimezone(ctx, userID, timezone)
}

// UpdateCarbsPerUnit updates how many grams of carbohydrates equal 1 bread unit (XE).
// grams must be in [1, 50].
func (uc *UseCase) UpdateCarbsPerUnit(ctx context.Context, id int64, grams float64) error {
	if grams < 1.0 || grams > 50.0 {
		return fmt.Errorf("user.UpdateCarbsPerUnit: grams %.1f out of range [1–50]", grams)
	}

	if err := uc.userRepo.UpdateCarbsPerUnit(ctx, id, grams); err != nil {
		return fmt.Errorf("user.UpdateCarbsPerUnit: %w", err)
	}

	return nil
}
