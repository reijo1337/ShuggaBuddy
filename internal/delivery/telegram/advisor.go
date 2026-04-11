package telegram

import (
	"context"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.uber.org/zap"

	"github.com/gmtantsevov/shuggabuddy/internal/domain"
)

// handleAdvisorShow runs the analysis and shows results.
func (h *Handler) handleAdvisorShow(ctx context.Context, cb *tgbotapi.CallbackQuery) {
	user, _, err := h.userUC.GetProfile(ctx, domain.ProviderTelegram, strconv.FormatInt(cb.From.ID, 10))
	if err != nil || user == nil {
		h.log.Error("handleAdvisorShow: failed to get user", zap.Error(err))
		h.reply(cb.Message.Chat.ID, h.loc.T("error_internal"))
		return
	}

	advice, err := h.advisorUC.Analyze(ctx, user.ID, time.Now())
	if err != nil {
		h.log.Error("handleAdvisorShow: analysis failed", zap.Error(err))
		h.reply(cb.Message.Chat.ID, h.loc.T("error_internal"))
		return
	}

	text := h.formatAdvice(advice, user)
	h.replyWithKeyboard(cb.Message.Chat.ID, text, h.backToMenuKeyboard())
}

// formatAdvice builds the advice message text.
func (h *Handler) formatAdvice(advice *domain.DoseAdvice, user *domain.User) string {
	var b strings.Builder
	b.WriteString(h.loc.T("advisor_title"))

	if advice.Basal == nil && advice.Bolus == nil {
		b.WriteString("\n\n")
		b.WriteString(h.loc.T("advisor_both_no_data"))
		return b.String()
	}

	switch {
	case advice.Basal != nil:
		b.WriteString(h.loc.T("advisor_basal_header"))
		b.WriteString("\n")
		b.WriteString(h.loc.T("advisor_basal_fasting", formatMmol(advice.Basal.FastingAvg), advice.Basal.FastingCount))
		b.WriteString("\n")

		switch advice.Basal.Trend {
		case domain.TrendHigh:
			b.WriteString(h.loc.T("advisor_basal_trend_high"))
		case domain.TrendLow:
			b.WriteString(h.loc.T("advisor_basal_trend_low"))
		case domain.TrendStable:
			b.WriteString(h.loc.T("advisor_basal_trend_stable"))
		}
		b.WriteString("\n")

		if advice.Basal.Trend == domain.TrendStable {
			b.WriteString(h.loc.T("advisor_basal_ok", formatDoseUnits(advice.Basal.CurrentDose)))
		} else {
			b.WriteString(h.loc.T("advisor_basal_suggest",
				formatDoseUnits(advice.Basal.CurrentDose),
				formatDoseUnits(advice.Basal.SuggestedDose)))
		}
	case user.BasalDose == 0:
		b.WriteString(h.loc.T("advisor_basal_header"))
		b.WriteString("\n")
		b.WriteString(h.loc.T("advisor_basal_no_dose"))
	default:
		b.WriteString(h.loc.T("advisor_basal_header"))
		b.WriteString("\n")
		b.WriteString(h.loc.T("advisor_basal_no_data"))
	}

	if advice.Bolus != nil {
		b.WriteString("\n")
		b.WriteString(h.loc.T("advisor_bolus_header"))
		b.WriteString("\n")

		if advice.Bolus.PostMealCount > 0 {
			b.WriteString(h.loc.T("advisor_bolus_postmeal", formatMmol(advice.Bolus.PostMealAvg), advice.Bolus.PostMealCount))
			b.WriteString("\n")
		}

		if advice.Bolus.ICRTrend == domain.TrendStable && advice.Bolus.ISFTrend == domain.TrendStable {
			b.WriteString(h.loc.T("advisor_bolus_stable"))
		} else {
			b.WriteString(h.loc.T("advisor_bolus_icr_change",
				formatDoseUnits(advice.Bolus.PreviousICR),
				formatDoseUnits(advice.Bolus.CurrentICR)))
			switch advice.Bolus.ICRTrend {
			case domain.TrendHigh:
				b.WriteString(h.loc.T("advisor_bolus_icr_more"))
			case domain.TrendLow:
				b.WriteString(h.loc.T("advisor_bolus_icr_less"))
			}
			b.WriteString("\n")

			if advice.Bolus.CurrentISF > 0 {
				b.WriteString(h.loc.T("advisor_bolus_isf_change",
					formatMmol(advice.Bolus.PreviousISF),
					formatMmol(advice.Bolus.CurrentISF)))
				switch advice.Bolus.ISFTrend {
				case domain.TrendHigh:
					b.WriteString(h.loc.T("advisor_bolus_isf_more"))
				case domain.TrendLow:
					b.WriteString(h.loc.T("advisor_bolus_isf_less"))
				}
			}
		}
	} else {
		b.WriteString("\n")
		b.WriteString(h.loc.T("advisor_bolus_header"))
		b.WriteString("\n")
		b.WriteString(h.loc.T("advisor_bolus_no_data"))
	}

	b.WriteString(h.loc.T("advisor_disclaimer"))

	return b.String()
}

// handleProfileBasalDoseStart begins basal dose input flow.
func (h *Handler) handleProfileBasalDoseStart(cb *tgbotapi.CallbackQuery) {
	sess := newSession(sessionProfile, stepProfileBasalDose)
	h.sessions.Store(cb.Message.Chat.ID, sess)
	h.replyWithKeyboard(cb.Message.Chat.ID, h.loc.T("profile_basal_dose_prompt"), h.backToMenuKeyboard())
}

// handleProfileBasalDoseInput processes basal dose text input.
func (h *Handler) handleProfileBasalDoseInput(ctx context.Context, msg *tgbotapi.Message, _ *Session) {
	text := strings.TrimSpace(msg.Text)
	dose, err := strconv.ParseFloat(text, 64)
	if err != nil || dose < 0.5 || dose > 200 {
		h.replyWithKeyboard(msg.Chat.ID, h.loc.T("profile_basal_dose_invalid"), h.backToMenuKeyboard())
		return
	}

	user, _, err := h.userUC.GetProfile(ctx, domain.ProviderTelegram, strconv.FormatInt(msg.From.ID, 10))
	if err != nil || user == nil {
		h.log.Error("handleProfileBasalDoseInput: failed to get user", zap.Error(err))
		h.reply(msg.Chat.ID, h.loc.T("error_internal"))
		return
	}

	if err := h.userUC.UpdateBasalDose(ctx, user.ID, dose); err != nil {
		h.log.Error("handleProfileBasalDoseInput: failed to save", zap.Error(err))
		h.reply(msg.Chat.ID, h.loc.T("error_internal"))
		return
	}

	h.sessions.Delete(msg.Chat.ID)
	h.replyWithKeyboard(msg.Chat.ID, h.loc.T("profile_basal_dose_saved", formatDoseUnits(dose)), h.backToMenuKeyboard())
}

// handleAdvisorIntervalMenu shows interval selection.
func (h *Handler) handleAdvisorIntervalMenu(cb *tgbotapi.CallbackQuery) {
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(h.loc.T("profile_advisor_interval_3"), "profile:advisor_interval:3"),
			tgbotapi.NewInlineKeyboardButtonData(h.loc.T("profile_advisor_interval_7"), "profile:advisor_interval:7"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(h.loc.T("profile_advisor_interval_14"), "profile:advisor_interval:14"),
			tgbotapi.NewInlineKeyboardButtonData(h.loc.T("profile_advisor_interval_custom"), "profile:advisor_interval:custom"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(h.loc.T("profile_advisor_interval_off"), "profile:advisor_interval:off"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(h.loc.T("btn_back_menu"), "menu:profile"),
		),
	)
	h.replyWithKeyboard(cb.Message.Chat.ID, h.loc.T("profile_advisor_interval_prompt"), keyboard)
}

// handleAdvisorIntervalSet saves a preset interval.
func (h *Handler) handleAdvisorIntervalSet(ctx context.Context, cb *tgbotapi.CallbackQuery, days int) {
	user, _, err := h.userUC.GetProfile(ctx, domain.ProviderTelegram, strconv.FormatInt(cb.From.ID, 10))
	if err != nil || user == nil {
		h.log.Error("handleAdvisorIntervalSet: failed to get user", zap.Error(err))
		h.reply(cb.Message.Chat.ID, h.loc.T("error_internal"))
		return
	}

	if err := h.userUC.UpdateAdvisorInterval(ctx, user.ID, days); err != nil {
		h.log.Error("handleAdvisorIntervalSet: failed to save", zap.Error(err))
		h.reply(cb.Message.Chat.ID, h.loc.T("error_internal"))
		return
	}

	if days == 0 {
		h.replyWithKeyboard(cb.Message.Chat.ID, h.loc.T("profile_advisor_interval_saved_off"), h.backToMenuKeyboard())
	} else {
		h.replyWithKeyboard(cb.Message.Chat.ID, h.loc.T("profile_advisor_interval_saved", days), h.backToMenuKeyboard())
	}
}

// handleAdvisorIntervalCustom starts custom interval input.
func (h *Handler) handleAdvisorIntervalCustom(cb *tgbotapi.CallbackQuery) {
	sess := newSession(sessionAdvisor, stepAdvisorInterval)
	h.sessions.Store(cb.Message.Chat.ID, sess)
	h.replyWithKeyboard(cb.Message.Chat.ID, h.loc.T("profile_advisor_interval_custom_prompt"), h.backToMenuKeyboard())
}

// handleAdvisorStep routes text input for advisor sessions.
func (h *Handler) handleAdvisorStep(ctx context.Context, msg *tgbotapi.Message, sess *Session) {
	if sess.Step == stepAdvisorInterval {
		h.handleAdvisorIntervalInput(ctx, msg, sess)
	}
}

// handleAdvisorIntervalInput processes custom interval text input.
func (h *Handler) handleAdvisorIntervalInput(ctx context.Context, msg *tgbotapi.Message, _ *Session) {
	text := strings.TrimSpace(msg.Text)
	days, err := strconv.Atoi(text)
	if err != nil || days < 1 || days > 90 {
		h.replyWithKeyboard(msg.Chat.ID, h.loc.T("profile_advisor_interval_custom_invalid"), h.backToMenuKeyboard())
		return
	}

	user, _, err := h.userUC.GetProfile(ctx, domain.ProviderTelegram, strconv.FormatInt(msg.From.ID, 10))
	if err != nil || user == nil {
		h.log.Error("handleAdvisorIntervalInput: failed to get user", zap.Error(err))
		h.reply(msg.Chat.ID, h.loc.T("error_internal"))
		return
	}

	if err := h.userUC.UpdateAdvisorInterval(ctx, user.ID, days); err != nil {
		h.log.Error("handleAdvisorIntervalInput: failed to save", zap.Error(err))
		h.reply(msg.Chat.ID, h.loc.T("error_internal"))
		return
	}

	h.sessions.Delete(msg.Chat.ID)
	h.replyWithKeyboard(msg.Chat.ID, h.loc.T("profile_advisor_interval_saved", days), h.backToMenuKeyboard())
}
