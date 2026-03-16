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
	SessionInsulin   = sessionInsulin

	StepGlucoseValue   = stepGlucoseValue
	StepFoodCarbs      = stepFoodCarbs
	StepFoodNote       = stepFoodNote
	StepFoodTime       = stepFoodTime
	StepCarbsUnitValue = stepCarbsUnitValue

	StepInsulinType    = stepInsulinType
	StepInsulinDose    = stepInsulinDose
	StepInsulinConfirm = stepInsulinConfirm
	StepInsulinDrug    = stepInsulinDrug

	SessionActivity = sessionActivity

	StepActivityType      = stepActivityType
	StepActivityCustom    = stepActivityCustom
	StepActivityDuration  = stepActivityDuration
	StepActivityTime      = stepActivityTime
	StepActivityIntensity = stepActivityIntensity
)

// SetSession stores a session directly for use in tests.
func (h *Handler) SetSession(chatID int64, sess *Session) {
	h.sessions.Store(chatID, sess)
}

var BuildMixedHistory = buildMixedHistory
