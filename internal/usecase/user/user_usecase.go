// Package user содержит бизнес-логику управления профилем пользователя.
package user

import (
	"context"
	"fmt"

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

	u := &domain.User{Units: domain.UnitsMmol}
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
