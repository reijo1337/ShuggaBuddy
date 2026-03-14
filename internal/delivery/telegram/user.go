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

// /start — приветствие + главное меню.
func (h *Handler) handleStart(ctx context.Context, msg *tgbotapi.Message) {
	h.sessions.Delete(msg.Chat.ID)

	user, acc, created, err := h.userUC.GetOrCreateUser(
		ctx,
		domain.ProviderTelegram,
		strconv.FormatInt(msg.From.ID, 10),
		msg.From.FirstName,
	)
	if err != nil {
		h.log.Error("handleStart: failed to get or create user", zap.Error(err))
		h.reply(msg.Chat.ID, h.loc.T("error_internal"))
		return
	}

	unitsLbl := h.unitsLabel(string(user.Units))
	carbsLbl := carbsPerUnitLabel(user.CarbsPerUnit)

	var greeting string
	if created {
		h.log.Info("new user registered",
			zap.Int64("user_id", user.ID),
			zap.String("provider", string(domain.ProviderTelegram)),
		)
		greeting = h.loc.T("welcome", acc.DisplayName)
	} else {
		greeting = h.loc.T("welcome_back", acc.DisplayName)
	}

	h.sendMenu(msg.Chat.ID, greeting+"\n\n"+h.loc.T("menu_title"), unitsLbl, carbsLbl)
}

// handleCallback маршрутизирует нажатия inline-кнопок.
func (h *Handler) handleCallback(ctx context.Context, cb *tgbotapi.CallbackQuery) {
	callback := tgbotapi.NewCallback(cb.ID, "")
	if _, err := h.bot.Request(callback); err != nil {
		h.log.Error("handleCallback: failed to answer callback", zap.Error(err))
	}

	switch cb.Data {
	case "menu:back":
		h.sessions.Delete(cb.Message.Chat.ID)
		h.handleMenuBack(ctx, cb)
	case "menu:profile":
		h.handleProfileCb(ctx, cb)
	case "menu:units":
		h.handleSetUnitsCb(ctx, cb)
	case "units:mmol":
		h.setUserUnits(ctx, cb, domain.UnitsMmol)
	case "units:mgdl":
		h.setUserUnits(ctx, cb, domain.UnitsMgdl)
	case "menu:glucose":
		h.handleGlucoseStart(cb)
	case "menu:last":
		h.handleLastCb(ctx, cb)
	case "menu:food":
		h.handleFoodStart(ctx, cb)
	case "food:unit:g", "food:unit:xe":
		h.handleFoodUnitToggle(ctx, cb)
	case "food:skip_note":
		h.handleFoodSkipNote(ctx, cb)
	case "food:time:now", "food:time:-15", "food:time:-30", "food:time:-60":
		h.handleFoodTimeQuick(ctx, cb)
	case "food:time:manual":
		h.handleFoodTimeManual(cb)
	case "menu:carbs_unit":
		h.handleCarbsUnitMenu(ctx, cb)
	case "carbs_unit:10":
		h.setCarbsPerUnit(ctx, cb, 10.0)
	case "carbs_unit:12":
		h.setCarbsPerUnit(ctx, cb, 12.0)
	case "carbs_unit:custom":
		h.handleCarbsUnitCustom(cb)
	case "menu:insulin":
		h.handleInsulinStart(cb)
	case "insulin:type:bolus":
		h.handleInsulinTypeSelect(cb, domain.InsulinTypeBolus)
	case "insulin:type:basal":
		h.handleInsulinTypeSelect(cb, domain.InsulinTypeBasal)
	case "insulin:skip_drug":
		h.handleInsulinSkipDrug(ctx, cb)
	case "insulin:confirm":
		h.handleInsulinConfirm(ctx, cb)
	case "insulin:cancel":
		h.sessions.Delete(cb.Message.Chat.ID)
		h.handleMenuBack(ctx, cb)
	}
}

// handleMenuBack — возврат в главное меню.
func (h *Handler) handleMenuBack(ctx context.Context, cb *tgbotapi.CallbackQuery) {
	user, _, err := h.userUC.GetProfile(ctx, domain.ProviderTelegram, strconv.FormatInt(cb.From.ID, 10))
	if err != nil || user == nil {
		h.log.Error("handleMenuBack: failed to get user", zap.Error(err))
		h.reply(cb.Message.Chat.ID, h.loc.T("error_internal"))
		return
	}

	unitsLbl := h.unitsLabel(string(user.Units))
	carbsLbl := carbsPerUnitLabel(user.CarbsPerUnit)
	h.sendMenu(cb.Message.Chat.ID, h.loc.T("menu_title"), unitsLbl, carbsLbl)
}

// handleProfileCb — профиль по нажатию кнопки.
func (h *Handler) handleProfileCb(ctx context.Context, cb *tgbotapi.CallbackQuery) {
	user, acc, err := h.userUC.GetProfile(ctx, domain.ProviderTelegram, strconv.FormatInt(cb.From.ID, 10))
	if err != nil || user == nil {
		h.log.Error("handleProfileCb: failed to get profile", zap.Error(err))
		h.reply(cb.Message.Chat.ID, h.loc.T("error_internal"))
		return
	}

	label := h.unitsLabel(string(user.Units))

	text := h.loc.T("profile",
		acc.DisplayName,
		label,
		user.CreatedAt.Format("02.01.2006"),
	)

	h.replyWithKeyboard(cb.Message.Chat.ID, text, h.backToMenuKeyboard())
}

// handleSetUnitsCb — выбор единиц измерения по нажатию кнопки.
func (h *Handler) handleSetUnitsCb(_ context.Context, cb *tgbotapi.CallbackQuery) {
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(h.loc.T("units_mmol"), "units:mmol"),
			tgbotapi.NewInlineKeyboardButtonData(h.loc.T("units_mgdl"), "units:mgdl"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(h.loc.T("btn_back_menu"), "menu:back"),
		),
	)

	h.replyWithKeyboard(cb.Message.Chat.ID, h.loc.T("setunits_prompt"), keyboard)
}

// setUserUnits обновляет единицы измерения и показывает меню.
func (h *Handler) setUserUnits(ctx context.Context, cb *tgbotapi.CallbackQuery, units domain.Units) {
	user, _, err := h.userUC.GetProfile(ctx, domain.ProviderTelegram, strconv.FormatInt(cb.From.ID, 10))
	if err != nil || user == nil {
		h.log.Error("setUserUnits: failed to resolve user", zap.Error(err))
		h.reply(cb.Message.Chat.ID, h.loc.T("error_internal"))
		return
	}

	if err := h.userUC.UpdateUnits(ctx, user.ID, units); err != nil {
		h.log.Error("setUserUnits: failed to update units", zap.Error(err))
		h.reply(cb.Message.Chat.ID, h.loc.T("error_internal"))
		return
	}

	label := h.unitsLabel(string(units))
	carbsLbl := carbsPerUnitLabel(user.CarbsPerUnit)
	text := h.loc.T("setunits_success", label) + "\n\n" + h.loc.T("menu_title")
	h.sendMenu(cb.Message.Chat.ID, text, label, carbsLbl)
}

// handleCarbsUnitMenu shows the ХЕ setting selection screen.
func (h *Handler) handleCarbsUnitMenu(_ context.Context, cb *tgbotapi.CallbackQuery) {
	gramSuffix := h.loc.T("units_gram_suffix")
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("10%s", gramSuffix), "carbs_unit:10"),
			tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("12%s", gramSuffix), "carbs_unit:12"),
			tgbotapi.NewInlineKeyboardButtonData(h.loc.T("carbs_unit_btn_custom"), "carbs_unit:custom"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(h.loc.T("btn_back_menu"), "menu:back"),
		),
	)
	h.replyWithKeyboard(cb.Message.Chat.ID, h.loc.T("carbs_unit_prompt"), keyboard)
}

// setCarbsPerUnit immediately saves a standard carbs-per-unit value.
func (h *Handler) setCarbsPerUnit(ctx context.Context, cb *tgbotapi.CallbackQuery, grams float64) {
	user, _, err := h.userUC.GetProfile(ctx, domain.ProviderTelegram, strconv.FormatInt(cb.From.ID, 10))
	if err != nil || user == nil {
		h.log.Error("setCarbsPerUnit: failed to resolve user", zap.Error(err))
		h.reply(cb.Message.Chat.ID, h.loc.T("error_internal"))
		return
	}

	if err := h.userUC.UpdateCarbsPerUnit(ctx, user.ID, grams); err != nil {
		h.log.Error("setCarbsPerUnit: failed to update", zap.Error(err))
		h.reply(cb.Message.Chat.ID, h.loc.T("error_internal"))
		return
	}

	carbsLbl := carbsPerUnitLabel(grams)
	text := h.loc.T("carbs_unit_saved", carbsLbl) + "\n\n" + h.loc.T("menu_title")
	unitsLbl := h.unitsLabel(string(user.Units))
	h.sendMenu(cb.Message.Chat.ID, text, unitsLbl, carbsLbl)
}

// handleCarbsUnitCustom starts a sessionCarbsUnit for custom input.
func (h *Handler) handleCarbsUnitCustom(cb *tgbotapi.CallbackQuery) {
	sess := newSession(sessionCarbsUnit, stepCarbsUnitValue)
	h.sessions.Store(cb.Message.Chat.ID, sess)
	h.replyWithKeyboard(cb.Message.Chat.ID, h.loc.T("carbs_unit_custom_prompt"), h.backToMenuKeyboard())
}

// handleCarbsUnitStep processes text input during a sessionCarbsUnit flow.
func (h *Handler) handleCarbsUnitStep(ctx context.Context, msg *tgbotapi.Message, _ *Session) {
	h.sessions.Delete(msg.Chat.ID)

	grams, err := strconv.ParseFloat(strings.TrimSpace(msg.Text), 64)
	if err != nil {
		h.replyWithKeyboard(msg.Chat.ID, h.loc.T("carbs_unit_invalid"), h.backToMenuKeyboard())
		return
	}

	user, _, err := h.userUC.GetProfile(ctx, domain.ProviderTelegram, strconv.FormatInt(msg.From.ID, 10))
	if err != nil || user == nil {
		h.log.Error("handleCarbsUnitStep: failed to resolve user", zap.Error(err))
		h.reply(msg.Chat.ID, h.loc.T("error_internal"))
		return
	}

	if err := h.userUC.UpdateCarbsPerUnit(ctx, user.ID, grams); err != nil {
		h.log.Error("handleCarbsUnitStep: failed to update", zap.Error(err))
		h.reply(msg.Chat.ID, h.loc.T("error_internal"))
		return
	}

	carbsLbl := carbsPerUnitLabel(grams)
	text := h.loc.T("carbs_unit_saved", carbsLbl) + "\n\n" + h.loc.T("menu_title")
	unitsLbl := h.unitsLabel(string(user.Units))
	h.sendMenu(msg.Chat.ID, text, unitsLbl, carbsLbl)
}
