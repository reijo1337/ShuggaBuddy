package telegram

import (
	"context"
	"time"

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
	UpdateCarbsPerUnit(ctx context.Context, userID int64, grams float64) error
	UpdateSettings(ctx context.Context, userID int64, targetMin, targetMax float64, basalDrug, basalTime string) error
	UpdateTimezone(ctx context.Context, userID int64, timezone string) error
	UpdateBolusDrug(ctx context.Context, userID int64, drug string) error
	UpdateBasalDose(ctx context.Context, userID int64, dose float64) error
	UpdateAdvisorInterval(ctx context.Context, userID int64, days int) error
}

//go:generate mockgen -destination=mocks/mock_glucose_usecase.go -package=tgmocks github.com/gmtantsevov/shuggabuddy/internal/delivery/telegram GlucoseUseCase

// GlucoseUseCase описывает бизнес-логику глюкозы, необходимую delivery-слою.
type GlucoseUseCase interface {
	SaveReading(ctx context.Context, userID int64, value float64, units domain.Units) error
	GetLastReadings(ctx context.Context, userID int64, limit int) ([]domain.GlucoseReading, error)
}

//go:generate mockgen -destination=mocks/mock_food_usecase.go -package=tgmocks github.com/gmtantsevov/shuggabuddy/internal/delivery/telegram FoodUseCase

// FoodUseCase описывает бизнес-логику записи еды, необходимую delivery-слою.
type FoodUseCase interface {
	SaveEntry(ctx context.Context, userID int64, carbsGrams float64, note string, eatenAt time.Time) error
	GetLastEntries(ctx context.Context, userID int64, limit int) ([]domain.FoodEntry, error)
}

//go:generate mockgen -destination=mocks/mock_insulin_usecase.go -package=tgmocks github.com/gmtantsevov/shuggabuddy/internal/delivery/telegram InsulinUseCase

// InsulinUseCase описывает бизнес-логику инсулина, необходимую delivery-слою.
type InsulinUseCase interface {
	SaveDose(ctx context.Context, userID int64, dose float64, insulinType domain.InsulinType, drug, source string) error
	GetLastDoses(ctx context.Context, userID int64, limit int) ([]domain.InsulinDose, error)
}

//go:generate mockgen -destination=mocks/mock_activity_usecase.go -package=tgmocks github.com/gmtantsevov/shuggabuddy/internal/delivery/telegram ActivityUseCase

// ActivityUseCase описывает бизнес-логику активности, необходимую delivery-слою.
type ActivityUseCase interface {
	SaveEntry(ctx context.Context, userID int64, activityType domain.ActivityType, customType string, durationMin int, intensity domain.Intensity, recordedAt time.Time, chatID int64) error
	GetLastEntries(ctx context.Context, userID int64, limit int) ([]domain.ActivityEntry, error)
	EvaluateImpact(activityType domain.ActivityType, durationMin int, intensity domain.Intensity) domain.GlycemicImpact
	AnalyzeLastActivities(ctx context.Context, userID int64, limit int) ([]domain.ActivityAnalysis, error)
}

//go:generate mockgen -destination=mocks/mock_note_usecase.go -package=tgmocks github.com/gmtantsevov/shuggabuddy/internal/delivery/telegram NoteUseCase

// NoteUseCase описывает бизнес-логику заметок, необходимую delivery-слою.
type NoteUseCase interface {
	SaveNote(ctx context.Context, userID int64, noteType domain.NoteType, wellbeing *domain.WellbeingValue, text *string) error
}

//go:generate mockgen -destination=mocks/mock_diary_usecase.go -package=tgmocks github.com/gmtantsevov/shuggabuddy/internal/delivery/telegram DiaryUseCase

// DiaryUseCase описывает бизнес-логику дневника, необходимую delivery-слою.
type DiaryUseCase interface {
	GetDayEntries(ctx context.Context, userID int64, date time.Time, loc *time.Location) ([]*domain.DiaryEntry, error)
}

//go:generate mockgen -destination=mocks/mock_bolus_usecase.go -package=tgmocks github.com/gmtantsevov/shuggabuddy/internal/delivery/telegram BolusUseCase

// BolusUseCase описывает бизнес-логику калькулятора болюса.
type BolusUseCase interface {
	Calculate(ctx context.Context, userID int64, currentGlucose, carbsGrams float64, now time.Time) (*domain.BolusRecommendation, error)
}

//go:generate mockgen -destination=mocks/mock_advisor_usecase.go -package=tgmocks github.com/gmtantsevov/shuggabuddy/internal/delivery/telegram DoseAdvisorUseCase

// DoseAdvisorUseCase описывает бизнес-логику рекомендаций по дозам.
type DoseAdvisorUseCase interface {
	Analyze(ctx context.Context, userID int64, now time.Time) (*domain.DoseAdvice, error)
}

//go:generate mockgen -destination=mocks/mock_cgm_usecase.go -package=tgmocks github.com/gmtantsevov/shuggabuddy/internal/delivery/telegram CGMUseCase

// CGMUseCase описывает бизнес-логику CGM-интеграции.
type CGMUseCase interface {
	AddConnection(ctx context.Context, userID int64, provider domain.CGMProvider, credential1, credential2 string) error
	TestConnection(ctx context.Context, userID int64) error
	RemoveConnection(ctx context.Context, userID int64) error
	GetConnection(ctx context.Context, userID int64) (*domain.CGMConnection, error)
}
