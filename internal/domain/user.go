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
	ID        int64     `json:"id"`
	Units     Units     `json:"units"`
	CreatedAt time.Time `json:"created_at"`
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
}

//go:generate mockgen -destination=mocks/mock_external_account_repository.go -package=mocks . ExternalAccountRepository

type ExternalAccountRepository interface {
	GetByProvider(ctx context.Context, provider Provider, externalID string) (*ExternalAccount, error)
	Create(ctx context.Context, account *ExternalAccount) error
}
