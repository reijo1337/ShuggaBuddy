package telegram

import (
	"context"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.uber.org/zap"

	"github.com/gmtantsevov/shuggabuddy/internal/domain"
	insulinuc "github.com/gmtantsevov/shuggabuddy/internal/usecase/insulin"
)

// handleInsulinStart начинает флоу записи инсулина.
func (h *Handler) handleInsulinStart(cb *tgbotapi.CallbackQuery) {
	sess := newSession(sessionInsulin, stepInsulinType)
	h.sessions.Store(cb.Message.Chat.ID, sess)

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(h.loc.T("insulin_btn_bolus"), "insulin:type:bolus"),
			tgbotapi.NewInlineKeyboardButtonData(h.loc.T("insulin_btn_basal"), "insulin:type:basal"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(h.loc.T("btn_back_menu"), "menu:back"),
		),
	)
	h.replyWithKeyboard(cb.Message.Chat.ID, h.loc.T("insulin_type_prompt"), keyboard)
}

// handleInsulinTypeSelect обрабатывает выбор типа инсулина.
func (h *Handler) handleInsulinTypeSelect(cb *tgbotapi.CallbackQuery, insulinType domain.InsulinType) {
	raw, ok := h.sessions.Load(cb.Message.Chat.ID)
	if !ok {
		return
	}
	sess := raw.(*Session)
	if sess.SType != sessionInsulin || sess.Step != stepInsulinType {
		return
	}
	sess.Data["type"] = string(insulinType)
	sess.Step = stepInsulinDose
	h.sessions.Store(cb.Message.Chat.ID, sess)

	h.replyWithKeyboard(cb.Message.Chat.ID, h.loc.T("insulin_dose_prompt"), h.backToMenuKeyboard())
}

// handleInsulinStep роутит текстовый ввод в зависимости от шага сессии.
func (h *Handler) handleInsulinStep(ctx context.Context, msg *tgbotapi.Message, sess *Session) {
	switch sess.Step {
	case stepInsulinDose:
		h.handleInsulinDoseInput(ctx, msg, sess)
	case stepInsulinDrug:
		h.handleInsulinDrugInput(ctx, msg, sess)
		// stepInsulinType и stepInsulinConfirm ждут колбэков, текст игнорируем.
	}
}

// handleInsulinDoseInput обрабатывает ввод дозы.
func (h *Handler) handleInsulinDoseInput(_ context.Context, msg *tgbotapi.Message, sess *Session) {
	text := strings.TrimSpace(msg.Text)
	dose, err := strconv.ParseFloat(text, 64)
	if err != nil || dose <= 0 {
		h.replyWithKeyboard(msg.Chat.ID, h.loc.T("insulin_invalid_dose"), h.backToMenuKeyboard())
		return
	}

	insulinType := domain.InsulinType(sess.Data["type"].(string))

	// Аномально большая доза — запрашиваем подтверждение.
	if insulinuc.IsAnomalousDose(dose, insulinType) {
		sess.Data["dose"] = dose
		sess.Step = stepInsulinConfirm
		h.sessions.Store(msg.Chat.ID, sess)

		typeLabel := h.insulinTypeLabel(insulinType)
		doseStr := formatDoseUnits(dose)
		warningText := h.loc.T("insulin_anomaly_warning", doseStr, typeLabel)

		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData(h.loc.T("insulin_btn_confirm"), "insulin:confirm"),
				tgbotapi.NewInlineKeyboardButtonData(h.loc.T("insulin_btn_cancel"), "insulin:cancel"),
			),
		)
		h.replyWithKeyboard(msg.Chat.ID, warningText, keyboard)
		return
	}

	// Доза нормальная — переходим к вводу препарата.
	sess.Data["dose"] = dose
	sess.Step = stepInsulinDrug
	h.sessions.Store(msg.Chat.ID, sess)
	h.replyWithKeyboard(msg.Chat.ID, h.loc.T("insulin_drug_prompt"), h.drugKeyboard())
}

// handleInsulinConfirm подтверждает аномальную дозу и переходит к вводу препарата.
func (h *Handler) handleInsulinConfirm(_ context.Context, cb *tgbotapi.CallbackQuery) {
	raw, ok := h.sessions.Load(cb.Message.Chat.ID)
	if !ok {
		return
	}
	sess := raw.(*Session)
	if sess.SType != sessionInsulin || sess.Step != stepInsulinConfirm {
		return
	}
	sess.Step = stepInsulinDrug
	h.sessions.Store(cb.Message.Chat.ID, sess)
	h.replyWithKeyboard(cb.Message.Chat.ID, h.loc.T("insulin_drug_prompt"), h.drugKeyboard())
}

// handleInsulinSkipDrug сохраняет дозу без указания препарата.
func (h *Handler) handleInsulinSkipDrug(ctx context.Context, cb *tgbotapi.CallbackQuery) {
	raw, ok := h.sessions.Load(cb.Message.Chat.ID)
	if !ok {
		return
	}
	sess := raw.(*Session)
	if sess.SType != sessionInsulin || sess.Step != stepInsulinDrug {
		return
	}
	h.saveInsulinDose(ctx, cb.Message.Chat.ID, cb.From.ID, sess, "")
}

// handleInsulinDrugInput обрабатывает ввод названия препарата.
func (h *Handler) handleInsulinDrugInput(ctx context.Context, msg *tgbotapi.Message, sess *Session) {
	drug := strings.TrimSpace(msg.Text)
	h.saveInsulinDose(ctx, msg.Chat.ID, msg.From.ID, sess, drug)
}

// saveInsulinDose сохраняет дозу и показывает подтверждение.
func (h *Handler) saveInsulinDose(ctx context.Context, chatID, fromID int64, sess *Session, drug string) {
	h.sessions.Delete(chatID)

	dose := sess.Data["dose"].(float64)
	insulinType := domain.InsulinType(sess.Data["type"].(string))

	user, _, err := h.userUC.GetProfile(ctx, domain.ProviderTelegram, strconv.FormatInt(fromID, 10))
	if err != nil || user == nil {
		h.log.Error("saveInsulinDose: failed to get user", zap.Error(err))
		h.reply(chatID, h.loc.T("error_internal"))
		return
	}

	if err := h.insulinUC.SaveDose(ctx, user.ID, dose, insulinType, drug); err != nil {
		h.log.Error("saveInsulinDose: failed to save dose", zap.Error(err))
		h.reply(chatID, h.loc.T("error_internal"))
		return
	}

	doseStr := formatDoseUnits(dose)
	typeLabel := h.insulinTypeLabel(insulinType)
	drugSuffix := ""
	if drug != "" {
		drugSuffix = h.loc.T("insulin_drug_suffix", drug)
	}
	h.replyWithKeyboard(chatID, h.loc.T("insulin_saved", doseStr, typeLabel, drugSuffix), h.backToMenuKeyboard())
}

// drugKeyboard строит клавиатуру для шага ввода препарата.
func (h *Handler) drugKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(h.loc.T("insulin_btn_skip_drug"), "insulin:skip_drug"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(h.loc.T("btn_back_menu"), "menu:back"),
		),
	)
}

// insulinTypeLabel возвращает локализованное название типа инсулина.
func (h *Handler) insulinTypeLabel(t domain.InsulinType) string {
	if t == domain.InsulinTypeBasal {
		return h.loc.T("insulin_type_basal")
	}
	return h.loc.T("insulin_type_bolus")
}

// formatDoseUnits форматирует дозу для отображения (без лишних нулей).
func formatDoseUnits(dose float64) string {
	return strconv.FormatFloat(dose, 'f', -1, 64)
}
