package telegram

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.uber.org/zap"

	"github.com/gmtantsevov/shuggabuddy/internal/domain"
)

func (h *Handler) handleAnalytics(ctx context.Context, cb *tgbotapi.CallbackQuery) {
	user, _, err := h.userUC.GetProfile(ctx, domain.ProviderTelegram, strconv.FormatInt(cb.From.ID, 10))
	if err != nil || user == nil {
		h.log.Error("handleAnalytics: failed to get user", zap.Error(err))
		h.reply(cb.Message.Chat.ID, h.loc.T("error_internal"))
		return
	}

	analyses, err := h.activityUC.AnalyzeLastActivities(ctx, user.ID, 5)
	if err != nil {
		h.log.Error("handleAnalytics: failed to analyze", zap.Error(err))
		h.reply(cb.Message.Chat.ID, h.loc.T("error_internal"))
		return
	}

	if len(analyses) == 0 {
		h.replyWithKeyboard(cb.Message.Chat.ID, h.loc.T("analytics_empty"), h.backToMenuKeyboard())
		return
	}

	var sb strings.Builder
	sb.WriteString(h.loc.T("analytics_header"))
	sb.WriteString("\n")

	for i := range analyses {
		a := &analyses[i]
		typeLabel := h.activityTypeLabel(a.Entry.ActivityType, a.Entry.CustomType)
		timeStr := a.Entry.RecordedAt.Format("02.01 15:04")

		if a.Delta != nil {
			fmt.Fprintf(&sb, h.loc.T("analytics_row_delta"),
				timeStr, typeLabel, a.Entry.DurationMin, *a.GlucBefore, *a.GlucAfter, *a.Delta)
		} else {
			fmt.Fprintf(&sb, h.loc.T("analytics_row_no_glucose"),
				timeStr, typeLabel, a.Entry.DurationMin)
		}
		sb.WriteString("\n")
	}

	h.replyWithKeyboard(cb.Message.Chat.ID, sb.String(), h.backToMenuKeyboard())
}
