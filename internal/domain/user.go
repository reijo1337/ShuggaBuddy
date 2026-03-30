// Package domain содержит бизнес-сущности и интерфейсы репозиториев.
// Этот слой не зависит от внешних фреймворков.
package domain

import (
	"context"
	"time"
)

type Units string

const (
	UnitsMmol Units = "mmol"
	UnitsMgdl Units = "mgdl"
)

const MmolToMgdl = 18.0182

// User представляет профиль пользователя бота.
type User struct {
	ID            int64     `json:"id"`
	Units         Units     `json:"units"`
	CarbsPerUnit  float64   `json:"carbs_per_unit"`  // grams of carbs per 1 bread unit (ХЕ), default 12
	TargetMinMmol float64   `json:"target_min_mmol"` // lower bound of target glucose range
	TargetMaxMmol float64   `json:"target_max_mmol"` // upper bound of target glucose range
	BasalDrug     string    `json:"basal_drug"`      // name of basal insulin drug, empty = not set
	BasalTime     string    `json:"basal_time"`      // preferred injection time (HH:MM), empty = not set
	Timezone      string    `json:"timezone"`        // IANA timezone name, e.g. "Europe/Moscow", default "UTC"
	CreatedAt     time.Time `json:"created_at"`
}

type Provider string

const ProviderTelegram Provider = "telegram"

// ExternalAccount связывает внешнего провайдера с внутренним пользователем.
type ExternalAccount struct {
	ID          int64     `json:"id"`
	UserID      int64     `json:"user_id"`
	Provider    Provider  `json:"provider"`
	ExternalID  string    `json:"external_id"`
	DisplayName string    `json:"display_name"`
	CreatedAt   time.Time `json:"created_at"`
}

//go:generate mockgen -destination=mocks/mock_user_repository.go -package=mocks . UserRepository

type UserRepository interface {
	GetByID(ctx context.Context, id int64) (*User, error)
	Create(ctx context.Context, user *User) error
	UpdateUnits(ctx context.Context, id int64, units Units) error
	UpdateCarbsPerUnit(ctx context.Context, id int64, grams float64) error
	UpdateSettings(ctx context.Context, userID int64, targetMin, targetMax float64, basalDrug, basalTime string) error
	UpdateTimezone(ctx context.Context, userID int64, timezone string) error
}

//go:generate mockgen -destination=mocks/mock_external_account_repository.go -package=mocks . ExternalAccountRepository

type ExternalAccountRepository interface {
	GetByProvider(ctx context.Context, provider Provider, externalID string) (*ExternalAccount, error)
	Create(ctx context.Context, account *ExternalAccount) error
}
