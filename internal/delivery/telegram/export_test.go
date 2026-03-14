package telegram

import (
	"context"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Экспортируем внутренние методы для тестов.

func (h *Handler) HandleStart(ctx context.Context, msg *tgbotapi.Message) {
	h.handleStart(ctx, msg)
}

func (h *Handler) HandleCallback(ctx context.Context, cb *tgbotapi.CallbackQuery) {
	h.handleCallback(ctx, cb)
}

func (h *Handler) HandleSessionInput(ctx context.Context, msg *tgbotapi.Message, sess *Session) {
	h.handleSessionInput(ctx, msg, sess)
}

// NewSession creates a Session for use in tests.
func NewSession(sType sessionType, step sessionStep) *Session {
	return newSession(sType, step)
}

// SessionGlucose and related constants re-exported for tests.
const (
	SessionGlucose   = sessionGlucose
	SessionFood      = sessionFood
	SessionCarbsUnit = sessionCarbsUnit

	StepGlucoseValue   = stepGlucoseValue
	StepFoodCarbs      = stepFoodCarbs
	StepFoodNote       = stepFoodNote
	StepFoodTime       = stepFoodTime
	StepCarbsUnitValue = stepCarbsUnitValue
)
