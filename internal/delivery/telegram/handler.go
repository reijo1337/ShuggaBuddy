// Package telegram содержит обработчики команд Telegram-бота.
package telegram

import (
	"context"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.uber.org/zap"

	"github.com/gmtantsevov/shuggabuddy/internal/i18n"
	"github.com/gmtantsevov/shuggabuddy/internal/usecase/glucose"
	"github.com/gmtantsevov/shuggabuddy/internal/usecase/user"
)

// Handler объединяет все обработчики команд Telegram-бота.
type Handler struct {
	bot    *tgbotapi.BotAPI
	userUC *user.UseCase
	glucUC *glucose.UseCase
	loc    *i18n.Localizer
	log    *zap.Logger
}

func NewHandler(
	bot *tgbotapi.BotAPI,
	userUC *user.UseCase,
	glucUC *glucose.UseCase,
	loc *i18n.Localizer,
	log *zap.Logger,
) *Handler {
	return &Handler{
		bot:    bot,
		userUC: userUC,
		glucUC: glucUC,
		loc:    loc,
		log:    log,
	}
}

func (h *Handler) Run(ctx context.Context) {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := h.bot.GetUpdatesChan(u)

	for {
		select {
		case <-ctx.Done():
			h.bot.StopReceivingUpdates()
			return
		case update := <-updates:
			if update.CallbackQuery != nil {
				h.handleCallback(ctx, update.CallbackQuery)
				continue
			}

			if update.Message == nil || !update.Message.IsCommand() {
				continue
			}

			switch update.Message.Command() {
			case "start":
				h.handleStart(ctx, update.Message)
			case "help":
				h.handleHelp(update.Message)
			case "profile":
				h.handleProfile(ctx, update.Message)
			case "setunits":
				h.handleSetUnits(update.Message)
			case "glucose":
				h.handleGlucose(ctx, update.Message)
			case "last":
				h.handleLast(ctx, update.Message)
			}
		}
	}
}

func (h *Handler) reply(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "Markdown"
	if _, err := h.bot.Send(msg); err != nil {
		h.log.Error("failed to send message", zap.Error(err), zap.Int64("chat_id", chatID))
	}
}

func (h *Handler) replyWithKeyboard(chatID int64, text string, keyboard tgbotapi.InlineKeyboardMarkup) {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyMarkup = keyboard
	if _, err := h.bot.Send(msg); err != nil {
		h.log.Error("failed to send message with keyboard", zap.Error(err), zap.Int64("chat_id", chatID))
	}
}
