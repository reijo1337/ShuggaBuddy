package telegram

import (
	"context"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/gmtantsevov/shuggabuddy/internal/domain"
)

// BotAPI абстрагирует операции Telegram Bot API, необходимые обработчикам.
type BotAPI interface {
	Send(c tgbotapi.Chattable) (tgbotapi.Message, error)
	Request(c tgbotapi.Chattable) (*tgbotapi.APIResponse, error)
	GetUpdatesChan(config tgbotapi.UpdateConfig) tgbotapi.UpdatesChannel
	StopReceivingUpdates()
}

//go:generate mockgen -destination=mocks/mock_user_usecase.go -package=tgmocks github.com/gmtantsevov/shuggabuddy/internal/delivery/telegram UserUseCase

// UserUseCase описывает бизнес-логику пользователей, необходимую delivery-слою.
type UserUseCase interface {
	GetOrCreateUser(ctx context.Context, provider domain.Provider, externalID, displayName string) (*domain.User, *domain.ExternalAccount, bool, error)
	GetProfile(ctx context.Context, provider domain.Provider, externalID string) (*domain.User, *domain.ExternalAccount, error)
	UpdateUnits(ctx context.Context, userID int64, units domain.Units) error
}

//go:generate mockgen -destination=mocks/mock_glucose_usecase.go -package=tgmocks github.com/gmtantsevov/shuggabuddy/internal/delivery/telegram GlucoseUseCase

// GlucoseUseCase описывает бизнес-логику глюкозы, необходимую delivery-слою.
type GlucoseUseCase interface {
	SaveReading(ctx context.Context, userID int64, value float64, units domain.Units) error
	GetLastReadings(ctx context.Context, userID int64, limit int) ([]domain.GlucoseReading, error)
}
