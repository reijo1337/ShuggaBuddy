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

// handleGlucose обрабатывает команду /glucose <value>.
func (h *Handler) handleGlucose(ctx context.Context, msg *tgbotapi.Message) {
	args := strings.TrimSpace(msg.CommandArguments())
	if args == "" {
		h.reply(msg.Chat.ID, h.loc.T("glucose_usage"))
		return
	}

	value, err := strconv.ParseFloat(args, 64)
	if err != nil {
		h.reply(msg.Chat.ID, h.loc.T("glucose_invalid"))
		return
	}

	user, _, err := h.userUC.GetProfile(ctx, domain.ProviderTelegram, strconv.FormatInt(msg.From.ID, 10))
	if err != nil || user == nil {
		h.log.Error("handleGlucose: failed to get user", zap.Error(err))
		h.reply(msg.Chat.ID, h.loc.T("error_internal"))
		return
	}

	if err := h.glucUC.SaveReading(ctx, user.ID, value, user.Units); err != nil {
		h.log.Warn("handleGlucose: invalid reading",
			zap.Error(err),
			zap.Float64("value", value),
		)
		if user.Units == domain.UnitsMgdl {
			h.reply(msg.Chat.ID, h.loc.T("glucose_out_of_range_mgdl"))
		} else {
			h.reply(msg.Chat.ID, h.loc.T("glucose_out_of_range_mmol"))
		}
		return
	}

	unitsLabel := h.loc.T("units_mmol")
	if user.Units == domain.UnitsMgdl {
		unitsLabel = h.loc.T("units_mgdl")
	}

	h.reply(msg.Chat.ID, h.loc.T("glucose_saved",
		glucose.FormatValue(value, domain.UnitsMmol), // отображаем как введено
		unitsLabel,
	))
}

// handleLast обрабатывает команду /last.
func (h *Handler) handleLast(ctx context.Context, msg *tgbotapi.Message) {
	user, _, err := h.userUC.GetProfile(ctx, domain.ProviderTelegram, strconv.FormatInt(msg.From.ID, 10))
	if err != nil || user == nil {
		h.log.Error("handleLast: failed to get user", zap.Error(err))
		h.reply(msg.Chat.ID, h.loc.T("error_internal"))
		return
	}

	readings, err := h.glucUC.GetLastReadings(ctx, user.ID, 5)
	if err != nil {
		h.log.Error("handleLast: failed to get readings", zap.Error(err))
		h.reply(msg.Chat.ID, h.loc.T("error_internal"))
		return
	}

	if len(readings) == 0 {
		h.reply(msg.Chat.ID, h.loc.T("last_empty"))
		return
	}

	unitsLabel := h.loc.T("units_mmol")
	if user.Units == domain.UnitsMgdl {
		unitsLabel = h.loc.T("units_mgdl")
	}

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

	h.reply(msg.Chat.ID, sb.String())
}
