// Package telegram содержит обработчики команд Telegram-бота.
package telegram

import (
	"context"
	"sync"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.uber.org/zap"

	"github.com/gmtantsevov/shuggabuddy/internal/i18n"
)

// Handler объединяет все обработчики команд Telegram-бота.
type Handler struct {
	bot    BotAPI
	userUC UserUseCase
	glucUC GlucoseUseCase
	loc    *i18n.Localizer
	log    *zap.Logger

	// waitingGlucose хранит chat ID пользователей, ожидающих ввод уровня сахара.
	waitingGlucose sync.Map
}

func NewHandler(
	bot BotAPI,
	userUC UserUseCase,
	glucUC GlucoseUseCase,
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

			if update.Message == nil {
				continue
			}

			if update.Message.IsCommand() && update.Message.Command() == "start" {
				h.handleStart(ctx, update.Message)
				continue
			}

			// Обработка текстового ввода уровня сахара.
			if _, ok := h.waitingGlucose.Load(update.Message.Chat.ID); ok {
				h.handleGlucoseInput(ctx, update.Message)
				continue
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
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = keyboard
	if _, err := h.bot.Send(msg); err != nil {
		h.log.Error("failed to send message with keyboard", zap.Error(err), zap.Int64("chat_id", chatID))
	}
}

func (h *Handler) menuKeyboard(unitsLabel string) tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(h.loc.T("btn_profile"), "menu:profile"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(
				h.loc.T("btn_units", unitsLabel), "menu:units",
			),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(h.loc.T("btn_glucose"), "menu:glucose"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(h.loc.T("btn_last"), "menu:last"),
		),
	)
}

func (h *Handler) backToMenuKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(h.loc.T("btn_back_menu"), "menu:back"),
		),
	)
}

func (h *Handler) sendMenu(chatID int64, text string, unitsLabel string) {
	h.replyWithKeyboard(chatID, text, h.menuKeyboard(unitsLabel))
}

func (h *Handler) unitsLabel(units string) string {
	if units == "mgdl" {
		return h.loc.T("units_mgdl")
	}
	return h.loc.T("units_mmol")
}
