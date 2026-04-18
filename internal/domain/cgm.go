package domain

import (
	"context"
	"time"
)

type CGMProvider string

const CGMProviderNightscout CGMProvider = "nightscout"

type CGMConnection struct {
	ID           int64       `json:"id"`
	UserID       int64       `json:"user_id"`
	Provider     CGMProvider `json:"provider"`
	BaseURL      string      `json:"base_url"`
	APIToken     string      `json:"api_token"`
	LastSyncedAt *time.Time  `json:"last_synced_at"`
	Active       bool        `json:"active"`
	CreatedAt    time.Time   `json:"created_at"`
}

//go:generate mockgen -destination=mocks/mock_cgm_repository.go -package=mocks . CGMConnectionRepository

type CGMConnectionRepository interface {
	Upsert(ctx context.Context, conn *CGMConnection) error
	GetByUserID(ctx context.Context, userID int64) (*CGMConnection, error)
	GetAllActive(ctx context.Context) ([]CGMConnection, error)
	UpdateLastSyncedAt(ctx context.Context, id int64, syncedAt time.Time) error
	Deactivate(ctx context.Context, id int64) error
	Delete(ctx context.Context, userID int64) error
}
