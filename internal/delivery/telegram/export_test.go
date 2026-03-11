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

func (h *Handler) HandleGlucoseInput(ctx context.Context, msg *tgbotapi.Message) {
	h.handleGlucoseInput(ctx, msg)
}
