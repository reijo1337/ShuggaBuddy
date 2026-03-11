package telegram

import (
	"context"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.uber.org/zap"

	"github.com/gmtantsevov/shuggabuddy/internal/domain"
	"github.com/gmtantsevov/shuggabuddy/internal/usecase/glucose"
)

// handleGlucoseStart переводит пользователя в режим ожидания ввода сахара.
func (h *Handler) handleGlucoseStart(cb *tgbotapi.CallbackQuery) {
	h.waitingGlucose.Store(cb.Message.Chat.ID, cb.From.ID)
	h.replyWithKeyboard(cb.Message.Chat.ID, h.loc.T("glucose_prompt"), h.backToMenuKeyboard())
}

// handleGlucoseInput обрабатывает текстовый ввод уровня сахара.
func (h *Handler) handleGlucoseInput(ctx context.Context, msg *tgbotapi.Message) {
	text := strings.TrimSpace(msg.Text)

	value, err := strconv.ParseFloat(text, 64)
	if err != nil {
		h.replyWithKeyboard(msg.Chat.ID, h.loc.T("glucose_invalid_short"), h.backToMenuKeyboard())
		return
	}

	user, _, err := h.userUC.GetProfile(ctx, domain.ProviderTelegram, strconv.FormatInt(msg.From.ID, 10))
	if err != nil || user == nil {
		h.log.Error("handleGlucoseInput: failed to get user", zap.Error(err))
		h.reply(msg.Chat.ID, h.loc.T("error_internal"))
		return
	}

	if err := h.glucUC.SaveReading(ctx, user.ID, value, user.Units); err != nil {
		h.log.Warn("handleGlucoseInput: invalid reading",
			zap.Error(err),
			zap.Float64("value", value),
		)
		var rangeMsg string
		if user.Units == domain.UnitsMgdl {
			rangeMsg = h.loc.T("glucose_out_of_range_mgdl")
		} else {
			rangeMsg = h.loc.T("glucose_out_of_range_mmol")
		}
		h.replyWithKeyboard(msg.Chat.ID, rangeMsg, h.backToMenuKeyboard())
		return
	}

	unitsLabel := h.unitsLabel(string(user.Units))

	// Пользователь вводит в своих единицах — отображаем как введено.
	var displayValue string
	if user.Units == domain.UnitsMgdl {
		displayValue = strconv.FormatFloat(value, 'f', 0, 64)
	} else {
		displayValue = strconv.FormatFloat(value, 'f', 1, 64)
	}

	h.replyWithKeyboard(
		msg.Chat.ID,
		h.loc.T("glucose_saved", displayValue, unitsLabel),
		h.backToMenuKeyboard(),
	)
}

// handleLastCb показывает последние 5 записей.
func (h *Handler) handleLastCb(ctx context.Context, cb *tgbotapi.CallbackQuery) {
	user, _, err := h.userUC.GetProfile(ctx, domain.ProviderTelegram, strconv.FormatInt(cb.From.ID, 10))
	if err != nil || user == nil {
		h.log.Error("handleLastCb: failed to get user", zap.Error(err))
		h.reply(cb.Message.Chat.ID, h.loc.T("error_internal"))
		return
	}

	readings, err := h.glucUC.GetLastReadings(ctx, user.ID, 5)
	if err != nil {
		h.log.Error("handleLastCb: failed to get readings", zap.Error(err))
		h.reply(cb.Message.Chat.ID, h.loc.T("error_internal"))
		return
	}

	if len(readings) == 0 {
		h.replyWithKeyboard(cb.Message.Chat.ID, h.loc.T("last_empty_short"), h.backToMenuKeyboard())
		return
	}

	unitsLabel := h.unitsLabel(string(user.Units))

	var sb strings.Builder
	sb.WriteString(h.loc.T("last_header"))
	sb.WriteString("\n")

	for _, r := range readings {
		sb.WriteString(h.loc.T("last_row",
			r.RecordedAt.Format("02.01 15:04"),
			glucose.FormatValue(r.ValueMmol, user.Units),
			unitsLabel,
		))
		sb.WriteString("\n")
	}

	h.replyWithKeyboard(cb.Message.Chat.ID, sb.String(), h.backToMenuKeyboard())
}
