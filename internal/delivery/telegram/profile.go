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

// timezoneOptions lists the IANA timezone names offered in the UI,
// paired with a short display label.
var timezoneOptions = []struct {
	iana  string
	label string
}{
	{"UTC", "UTC (±0)"},
	{"Europe/Kaliningrad", "Калининград (UTC+2)"},
	{"Europe/Moscow", "Москва (UTC+3)"},
	{"Europe/Samara", "Самара (UTC+4)"},
	{"Asia/Yekaterinburg", "Екатеринбург (UTC+5)"},
	{"Asia/Omsk", "Омск (UTC+6)"},
	{"Asia/Krasnoyarsk", "Красноярск (UTC+7)"},
	{"Asia/Irkutsk", "Иркутск (UTC+8)"},
	{"Asia/Yakutsk", "Якутск (UTC+9)"},
	{"Asia/Vladivostok", "Владивосток (UTC+10)"},
	{"Asia/Magadan", "Магадан (UTC+11)"},
	{"Asia/Kamchatka", "Камчатка (UTC+12)"},
}

// handleTimezoneMenu shows a list of timezone buttons.
func (h *Handler) handleTimezoneMenu(cb *tgbotapi.CallbackQuery) {
	rows := make([][]tgbotapi.InlineKeyboardButton, 0, len(timezoneOptions)+1)
	for _, tz := range timezoneOptions {
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(tz.label, "profile:timezone:set:"+tz.iana),
		))
	}
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData(h.loc.T("btn_back_menu"), "menu:profile"),
	))
	keyboard := tgbotapi.NewInlineKeyboardMarkup(rows...)
	h.replyWithKeyboard(cb.Message.Chat.ID, h.loc.T("profile_timezone_prompt"), keyboard)
}

// handleTimezoneSet saves the selected timezone and returns to profile.
func (h *Handler) handleTimezoneSet(ctx context.Context, cb *tgbotapi.CallbackQuery, iana string) {
	user, _, err := h.userUC.GetProfile(ctx, domain.ProviderTelegram, strconv.FormatInt(cb.From.ID, 10))
	if err != nil || user == nil {
		h.log.Error("handleTimezoneSet: failed to get user", zap.Error(err))
		h.reply(cb.Message.Chat.ID, h.loc.T("error_internal"))
		return
	}

	if err := h.userUC.UpdateTimezone(ctx, user.ID, iana); err != nil {
		h.log.Error("handleTimezoneSet: failed to update timezone", zap.Error(err))
		h.reply(cb.Message.Chat.ID, h.loc.T("error_internal"))
		return
	}

	h.sendProfileView(ctx, cb.Message.Chat.ID, cb.From.ID)
}

// formatMmol formats a mmol/L value to 1 decimal place.
func formatMmol(v float64) string {
	return strconv.FormatFloat(v, 'f', 1, 64)
}

// handleProfileStep routes profile session text input to the correct step handler.
func (h *Handler) handleProfileStep(ctx context.Context, msg *tgbotapi.Message, sess *Session) {
	switch sess.Step {
	case stepProfileTargetMin:
		h.handleProfileTargetMinStep(ctx, msg, sess)
	case stepProfileTargetMax:
		h.handleProfileTargetMaxStep(ctx, msg, sess)
	case stepProfileBasalDrug:
		h.handleProfileBasalDrugStep(ctx, msg, sess)
	case stepProfileBasalTime:
		h.handleProfileBasalTimeStep(ctx, msg, sess)
	}
}

// handleProfileTargetRangeStart begins the target range flow.
func (h *Handler) handleProfileTargetRangeStart(cb *tgbotapi.CallbackQuery) {
	sess := newSession(sessionProfile, stepProfileTargetMin)
	h.sessions.Store(cb.Message.Chat.ID, sess)
	h.replyWithKeyboard(cb.Message.Chat.ID, h.loc.T("profile_target_min_prompt"), h.backToMenuKeyboard())
}

// handleProfileTargetMinStep processes the minimum target glucose value.
func (h *Handler) handleProfileTargetMinStep(_ context.Context, msg *tgbotapi.Message, sess *Session) {
	v, err := strconv.ParseFloat(strings.TrimSpace(msg.Text), 64)
	if err != nil || v < 1.0 || v > 33.3 {
		h.replyWithKeyboard(msg.Chat.ID, h.loc.T("profile_target_invalid"), h.backToMenuKeyboard())
		return
	}
	sess.Data["target_min"] = v
	sess.Step = stepProfileTargetMax
	h.sessions.Store(msg.Chat.ID, sess)
	h.replyWithKeyboard(msg.Chat.ID, h.loc.T("profile_target_max_prompt"), h.backToMenuKeyboard())
}

// handleProfileTargetMaxStep processes the maximum target glucose value and saves.
func (h *Handler) handleProfileTargetMaxStep(ctx context.Context, msg *tgbotapi.Message, sess *Session) {
	v, err := strconv.ParseFloat(strings.TrimSpace(msg.Text), 64)
	if err != nil || v < 1.0 || v > 33.3 {
		h.replyWithKeyboard(msg.Chat.ID, h.loc.T("profile_target_invalid"), h.backToMenuKeyboard())
		return
	}

	minVal := sess.Data["target_min"].(float64)
	if v <= minVal {
		h.replyWithKeyboard(msg.Chat.ID, h.loc.T("profile_target_range_invalid"), h.backToMenuKeyboard())
		return
	}

	user, _, err := h.userUC.GetProfile(ctx, domain.ProviderTelegram, strconv.FormatInt(msg.From.ID, 10))
	if err != nil || user == nil {
		h.log.Error("handleProfileTargetMaxStep: failed to get user", zap.Error(err))
		h.reply(msg.Chat.ID, h.loc.T("error_internal"))
		return
	}

	if err := h.userUC.UpdateSettings(ctx, user.ID, minVal, v, user.BasalDrug, user.BasalTime); err != nil {
		h.log.Error("handleProfileTargetMaxStep: failed to update settings", zap.Error(err))
		h.reply(msg.Chat.ID, h.loc.T("error_internal"))
		return
	}

	h.sessions.Delete(msg.Chat.ID)
	text := fmt.Sprintf(h.loc.T("profile_target_saved"), formatMmol(minVal), formatMmol(v))
	h.replyWithKeyboard(msg.Chat.ID, text, h.backToMenuKeyboard())
}

// basalSkipKeyboard builds the keyboard with a skip button for basal flow steps.
func (h *Handler) basalSkipKeyboard(skipCallback string) tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(h.loc.T("profile_basal_skip"), skipCallback),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(h.loc.T("btn_back_menu"), "menu:back"),
		),
	)
}

// handleProfileBasalStart begins the basal insulin flow.
func (h *Handler) handleProfileBasalStart(cb *tgbotapi.CallbackQuery) {
	sess := newSession(sessionProfile, stepProfileBasalDrug)
	h.sessions.Store(cb.Message.Chat.ID, sess)
	h.replyWithKeyboard(cb.Message.Chat.ID, h.loc.T("profile_basal_drug_prompt"), h.basalSkipKeyboard("profile:basal:skip_drug"))
}

// handleProfileBasalSkipDrug skips drug input and advances to time step.
func (h *Handler) handleProfileBasalSkipDrug(cb *tgbotapi.CallbackQuery) {
	raw, ok := h.sessions.Load(cb.Message.Chat.ID)
	if !ok {
		return
	}
	sess := raw.(*Session)
	sess.Data["basal_drug"] = ""
	sess.Step = stepProfileBasalTime
	h.sessions.Store(cb.Message.Chat.ID, sess)
	h.replyWithKeyboard(cb.Message.Chat.ID, h.loc.T("profile_basal_time_prompt"), h.basalSkipKeyboard("profile:basal:skip_time"))
}

// handleProfileBasalDrugStep saves the drug name and advances to time step.
func (h *Handler) handleProfileBasalDrugStep(_ context.Context, msg *tgbotapi.Message, sess *Session) {
	sess.Data["basal_drug"] = strings.TrimSpace(msg.Text)
	sess.Step = stepProfileBasalTime
	h.sessions.Store(msg.Chat.ID, sess)
	h.replyWithKeyboard(msg.Chat.ID, h.loc.T("profile_basal_time_prompt"), h.basalSkipKeyboard("profile:basal:skip_time"))
}

// handleProfileBasalSkipTime skips time input and saves basal settings.
func (h *Handler) handleProfileBasalSkipTime(ctx context.Context, cb *tgbotapi.CallbackQuery) {
	raw, ok := h.sessions.Load(cb.Message.Chat.ID)
	if !ok {
		return
	}
	sess := raw.(*Session)
	drug, _ := sess.Data["basal_drug"].(string)
	h.saveBasalSettings(ctx, cb.Message.Chat.ID, cb.From.ID, drug, "")
}

// handleProfileBasalTimeStep validates the time input and saves basal settings.
func (h *Handler) handleProfileBasalTimeStep(ctx context.Context, msg *tgbotapi.Message, sess *Session) {
	text := strings.TrimSpace(msg.Text)
	if !isValidHHMM(text) {
		h.replyWithKeyboard(msg.Chat.ID, h.loc.T("profile_basal_time_prompt"), h.basalSkipKeyboard("profile:basal:skip_time"))
		return
	}
	drug, _ := sess.Data["basal_drug"].(string)
	h.saveBasalSettings(ctx, msg.Chat.ID, msg.From.ID, drug, text)
}

// isValidHHMM checks that s is in HH:MM format with hour 0-23 and minute 0-59.
func isValidHHMM(s string) bool {
	parts := strings.Split(s, ":")
	if len(parts) != 2 {
		return false
	}
	h, err1 := strconv.Atoi(parts[0])
	m, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil {
		return false
	}
	return h >= 0 && h <= 23 && m >= 0 && m <= 59 && len(parts[0]) == 2 && len(parts[1]) == 2
}

// saveBasalSettings persists basal drug and time, then clears the session.
func (h *Handler) saveBasalSettings(ctx context.Context, chatID, fromID int64, basalDrug, basalTime string) {
	user, _, err := h.userUC.GetProfile(ctx, domain.ProviderTelegram, strconv.FormatInt(fromID, 10))
	if err != nil || user == nil {
		h.log.Error("saveBasalSettings: failed to get user", zap.Error(err))
		h.reply(chatID, h.loc.T("error_internal"))
		return
	}

	if err := h.userUC.UpdateSettings(ctx, user.ID, user.TargetMinMmol, user.TargetMaxMmol, basalDrug, basalTime); err != nil {
		h.log.Error("saveBasalSettings: failed to update settings", zap.Error(err))
		h.reply(chatID, h.loc.T("error_internal"))
		return
	}

	h.sessions.Delete(chatID)
	h.replyWithKeyboard(chatID, h.loc.T("profile_basal_saved"), h.backToMenuKeyboard())
}
