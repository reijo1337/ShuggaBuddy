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

// handleBolusMethodChoice показывает выбор: ввести дозу вручную или калькулятор.
func (h *Handler) handleBolusMethodChoice(cb *tgbotapi.CallbackQuery) {
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(h.loc.T("insulin_btn_manual"), "insulin:manual"),
			tgbotapi.NewInlineKeyboardButtonData(h.loc.T("insulin_btn_calc"), "bolus:start"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(h.loc.T("btn_back_menu"), "menu:back"),
		),
	)
	h.replyWithKeyboard(cb.Message.Chat.ID, h.loc.T("insulin_dose_prompt"), keyboard)
}

// handleBolusStart начинает флоу калькулятора.
func (h *Handler) handleBolusStart(ctx context.Context, cb *tgbotapi.CallbackQuery) {
	user, _, err := h.userUC.GetProfile(ctx, domain.ProviderTelegram, strconv.FormatInt(cb.From.ID, 10))
	if err != nil || user == nil {
		h.log.Error("handleBolusStart: failed to get user", zap.Error(err))
		h.reply(cb.Message.Chat.ID, h.loc.T("error_internal"))
		return
	}

	if user.BolusDrug == "" {
		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData(h.loc.T("bolus_err_no_drug_btn"), "profile:bolus_drug"),
			),
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData(h.loc.T("btn_back_menu"), "menu:back"),
			),
		)
		h.replyWithKeyboard(cb.Message.Chat.ID, h.loc.T("bolus_err_no_drug"), keyboard)
		return
	}

	readings, err := h.glucUC.GetLastReadings(ctx, user.ID, 1)
	if err != nil {
		h.log.Error("handleBolusStart: failed to get glucose", zap.Error(err))
		h.reply(cb.Message.Chat.ID, h.loc.T("error_internal"))
		return
	}

	sess := newSession(sessionBolus, stepBolusGlucose)
	sess.Data["user_id"] = user.ID
	sess.Data["bolus_drug"] = user.BolusDrug
	sess.Data["units"] = string(user.Units)

	if len(readings) > 0 {
		age := time.Since(readings[0].RecordedAt)
		if age.Minutes() <= 30 {
			sess.Data["suggested_glucose"] = readings[0].ValueMmol
			h.sessions.Store(cb.Message.Chat.ID, sess)

			valueStr := formatMmol(readings[0].ValueMmol)
			unitsLabel := h.unitsLabel(string(user.Units))
			ageStr := h.formatDuration(age)

			keyboard := tgbotapi.NewInlineKeyboardMarkup(
				tgbotapi.NewInlineKeyboardRow(
					tgbotapi.NewInlineKeyboardButtonData(h.loc.T("bolus_glucose_btn_confirm"), "bolus:glucose:confirm"),
					tgbotapi.NewInlineKeyboardButtonData(h.loc.T("bolus_glucose_btn_manual"), "bolus:glucose:manual"),
				),
				tgbotapi.NewInlineKeyboardRow(
					tgbotapi.NewInlineKeyboardButtonData(h.loc.T("btn_back_menu"), "menu:back"),
				),
			)
			h.replyWithKeyboard(cb.Message.Chat.ID, h.loc.T("bolus_glucose_confirm", valueStr, unitsLabel, ageStr), keyboard)
			return
		}
	}

	sess.Step = stepBolusGlucoseManual
	h.sessions.Store(cb.Message.Chat.ID, sess)
	h.replyWithKeyboard(cb.Message.Chat.ID, h.loc.T("bolus_glucose_prompt"), h.backToMenuKeyboard())
}

// handleBolusGlucoseConfirm использует последнее показание.
func (h *Handler) handleBolusGlucoseConfirm(cb *tgbotapi.CallbackQuery) {
	raw, ok := h.sessions.Load(cb.Message.Chat.ID)
	if !ok {
		return
	}
	sess := raw.(*Session)
	if sess.SType != sessionBolus || sess.Step != stepBolusGlucose {
		return
	}

	glucose, ok := sess.Data["suggested_glucose"].(float64)
	if !ok {
		return
	}

	sess.Data["glucose"] = glucose
	sess.Step = stepBolusCarbs
	h.sessions.Store(cb.Message.Chat.ID, sess)
	h.replyWithKeyboard(cb.Message.Chat.ID, h.loc.T("bolus_carbs_prompt"), h.backToMenuKeyboard())
}

// handleBolusGlucoseManual переключает на ручной ввод глюкозы.
func (h *Handler) handleBolusGlucoseManual(cb *tgbotapi.CallbackQuery) {
	raw, ok := h.sessions.Load(cb.Message.Chat.ID)
	if !ok {
		return
	}
	sess := raw.(*Session)
	if sess.SType != sessionBolus {
		return
	}

	sess.Step = stepBolusGlucoseManual
	h.sessions.Store(cb.Message.Chat.ID, sess)
	h.replyWithKeyboard(cb.Message.Chat.ID, h.loc.T("bolus_glucose_prompt"), h.backToMenuKeyboard())
}

// handleBolusStep роутит текстовый ввод в зависимости от шага.
func (h *Handler) handleBolusStep(ctx context.Context, msg *tgbotapi.Message, sess *Session) {
	switch sess.Step {
	case stepBolusGlucoseManual:
		h.handleBolusGlucoseInput(ctx, msg, sess)
	case stepBolusCarbs:
		h.handleBolusCarbsInput(ctx, msg, sess)
	}
}

// handleBolusGlucoseInput обрабатывает текстовый ввод глюкозы.
func (h *Handler) handleBolusGlucoseInput(_ context.Context, msg *tgbotapi.Message, sess *Session) {
	text := strings.TrimSpace(msg.Text)
	value, err := strconv.ParseFloat(text, 64)
	if err != nil {
		h.replyWithKeyboard(msg.Chat.ID, h.loc.T("glucose_invalid_short"), h.backToMenuKeyboard())
		return
	}

	units := domain.Units(sess.Data["units"].(string))
	var valueMmol float64
	if units == domain.UnitsMgdl {
		if value < 18 || value > 600 {
			h.replyWithKeyboard(msg.Chat.ID, h.loc.T("glucose_out_of_range_mgdl"), h.backToMenuKeyboard())
			return
		}
		valueMmol = value / domain.MmolToMgdl
	} else {
		if value < 1.0 || value > 33.3 {
			h.replyWithKeyboard(msg.Chat.ID, h.loc.T("glucose_out_of_range_mmol"), h.backToMenuKeyboard())
			return
		}
		valueMmol = value
	}

	sess.Data["glucose"] = valueMmol
	sess.Step = stepBolusCarbs
	h.sessions.Store(msg.Chat.ID, sess)
	h.replyWithKeyboard(msg.Chat.ID, h.loc.T("bolus_carbs_prompt"), h.backToMenuKeyboard())
}

// handleBolusCarbsInput обрабатывает ввод углеводов и выполняет расчёт.
func (h *Handler) handleBolusCarbsInput(ctx context.Context, msg *tgbotapi.Message, sess *Session) {
	text := strings.TrimSpace(msg.Text)
	carbs, err := strconv.ParseFloat(text, 64)
	if err != nil || carbs < 0.1 || carbs > 500 {
		h.replyWithKeyboard(msg.Chat.ID, h.loc.T("food_invalid_carbs"), h.backToMenuKeyboard())
		return
	}

	glucose := sess.Data["glucose"].(float64)
	userID := sess.Data["user_id"].(int64)

	rec, err := h.bolusUC.Calculate(ctx, userID, glucose, carbs, time.Now())
	if err != nil {
		h.log.Warn("handleBolusCarbsInput: calculation failed", zap.Error(err))
		h.sessions.Delete(msg.Chat.ID)
		h.replyWithKeyboard(msg.Chat.ID, h.loc.T("bolus_err_insufficient_data"), h.backToMenuKeyboard())
		return
	}

	sess.Data["recommendation"] = rec
	h.sessions.Store(msg.Chat.ID, sess)

	var resultText string
	if rec.TotalDose == 0 {
		resultText = h.loc.T("bolus_result_zero")
	} else {
		resultText = h.loc.T("bolus_result", formatDoseUnits(rec.TotalDose))
	}

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(h.loc.T("bolus_btn_details"), "bolus:details"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(h.loc.T("bolus_btn_save"), "bolus:save"),
			tgbotapi.NewInlineKeyboardButtonData(h.loc.T("bolus_btn_cancel"), "bolus:cancel"),
		),
	)
	h.replyWithKeyboard(msg.Chat.ID, resultText, keyboard)
}

// handleBolusDetails показывает пошаговый расчёт.
func (h *Handler) handleBolusDetails(cb *tgbotapi.CallbackQuery) {
	raw, ok := h.sessions.Load(cb.Message.Chat.ID)
	if !ok {
		return
	}
	sess := raw.(*Session)
	if sess.SType != sessionBolus {
		return
	}
	rec, ok := sess.Data["recommendation"].(*domain.BolusRecommendation)
	if !ok {
		return
	}

	text := h.loc.T("bolus_details",
		formatDoseUnits(rec.FoodDose),
		formatDoseUnits(rec.ICR),
		formatDoseUnits(rec.CorrectionDose),
		formatMmol(rec.ISF),
		formatDoseUnits(rec.IOB),
		formatDoseUnits(rec.TotalDose),
	)
	h.reply(cb.Message.Chat.ID, text)
}

// handleBolusSave сохраняет рекомендацию как запись инсулина.
func (h *Handler) handleBolusSave(ctx context.Context, cb *tgbotapi.CallbackQuery) {
	raw, ok := h.sessions.Load(cb.Message.Chat.ID)
	if !ok {
		return
	}
	sess := raw.(*Session)
	if sess.SType != sessionBolus {
		return
	}
	rec, ok := sess.Data["recommendation"].(*domain.BolusRecommendation)
	if !ok {
		return
	}

	h.sessions.Delete(cb.Message.Chat.ID)

	userID := sess.Data["user_id"].(int64)
	drugKey := sess.Data["bolus_drug"].(string)
	drugProfile := domain.BolusInsulinCatalog[drugKey]

	if err := h.insulinUC.SaveDose(ctx, userID, rec.TotalDose, domain.InsulinTypeBolus, drugProfile.Name, "bolus_calculator"); err != nil {
		h.log.Error("handleBolusSave: failed to save", zap.Error(err))
		h.reply(cb.Message.Chat.ID, h.loc.T("error_internal"))
		return
	}

	text := h.loc.T("bolus_saved", formatDoseUnits(rec.TotalDose), drugProfile.Name)
	h.replyWithKeyboard(cb.Message.Chat.ID, text, h.backToMenuKeyboard())
}

// formatDuration formats a duration as a human-readable localized string.
func (h *Handler) formatDuration(d time.Duration) string {
	mins := int(d.Minutes())
	if mins < 1 {
		return h.loc.T("duration_just_now")
	}
	return h.loc.T("duration_minutes", mins)
}
