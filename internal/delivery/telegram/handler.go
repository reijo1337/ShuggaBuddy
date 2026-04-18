// Package telegram содержит обработчики команд Telegram-бота.
package telegram

import (
	"context"
	"strconv"
	"sync"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.uber.org/zap"

	"github.com/gmtantsevov/shuggabuddy/internal/i18n"
)

// Handler объединяет все обработчики команд Telegram-бота.
type Handler struct {
	bot        BotAPI
	userUC     UserUseCase
	glucUC     GlucoseUseCase
	foodUC     FoodUseCase
	insulinUC  InsulinUseCase
	activityUC ActivityUseCase
	noteUC     NoteUseCase
	diaryUC    DiaryUseCase
	bolusUC    BolusUseCase
	advisorUC  DoseAdvisorUseCase
	cgmUC      CGMUseCase
	loc        *i18n.Localizer
	log        *zap.Logger

	// sessions хранит активные диалоговые сессии, ключ — chat ID.
	sessions sync.Map
}

func NewHandler(
	bot BotAPI,
	userUC UserUseCase,
	glucUC GlucoseUseCase,
	foodUC FoodUseCase,
	insulinUC InsulinUseCase,
	activityUC ActivityUseCase,
	noteUC NoteUseCase,
	diaryUC DiaryUseCase,
	bolusUC BolusUseCase,
	advisorUC DoseAdvisorUseCase,
	cgmUC CGMUseCase,
	loc *i18n.Localizer,
	log *zap.Logger,
) *Handler {
	return &Handler{
		bot:        bot,
		userUC:     userUC,
		glucUC:     glucUC,
		foodUC:     foodUC,
		insulinUC:  insulinUC,
		activityUC: activityUC,
		noteUC:     noteUC,
		diaryUC:    diaryUC,
		bolusUC:    bolusUC,
		advisorUC:  advisorUC,
		cgmUC:      cgmUC,
		loc:        loc,
		log:        log,
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

			// Route text to the active session for this chat, if any.
			if sess, ok := h.sessions.Load(update.Message.Chat.ID); ok {
				h.handleSessionInput(ctx, update.Message, sess.(*Session))
				continue
			}
		}
	}
}

// handleSessionInput dispatches text input to the correct flow based on session type.
func (h *Handler) handleSessionInput(ctx context.Context, msg *tgbotapi.Message, sess *Session) {
	switch sess.SType {
	case sessionGlucose:
		h.handleGlucoseStep(ctx, msg, sess)
	case sessionFood:
		h.handleFoodStep(ctx, msg, sess)
	case sessionCarbsUnit:
		h.handleCarbsUnitStep(ctx, msg, sess)
	case sessionInsulin:
		h.handleInsulinStep(ctx, msg, sess)
	case sessionActivity:
		h.handleActivityStep(ctx, msg, sess)
	case sessionNote:
		h.handleNoteText(ctx, msg, sess)
	case sessionDiary:
		h.handleDiaryText(ctx, msg, sess)
	case sessionProfile:
		h.handleProfileStep(ctx, msg, sess)
	case sessionBolus:
		h.handleBolusStep(ctx, msg, sess)
	case sessionAdvisor:
		h.handleAdvisorStep(ctx, msg, sess)
	case sessionCGM:
		h.handleCGMStep(ctx, msg, sess)
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

// menuKeyboard builds the main menu.
func (h *Handler) menuKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(h.loc.T("btn_profile"), "menu:profile"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(h.loc.T("btn_new_entry"), "menu:new_entry"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(h.loc.T("btn_analytics"), "menu:analytics"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(h.loc.T("btn_advisor"), "menu:advisor"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(h.loc.T("btn_diary"), "menu:diary"),
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

func (h *Handler) sendMenu(chatID int64, text string) {
	h.replyWithKeyboard(chatID, text, h.menuKeyboard())
}

func (h *Handler) unitsLabel(units string) string {
	if units == "mgdl" {
		return h.loc.T("units_mgdl")
	}
	return h.loc.T("units_mmol")
}

// carbsPerUnitLabel formats the carbs-per-unit value for display (strips trailing zeros).
// Uses strconv.FormatFloat with 'f' format and prec=-1 (same as FormatCarbs in food usecase).
func carbsPerUnitLabel(grams float64) string {
	return strconv.FormatFloat(grams, 'f', -1, 64)
}
