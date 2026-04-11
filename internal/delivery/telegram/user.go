package telegram

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

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

	h.sendMenu(msg.Chat.ID, greeting+"\n\n"+h.loc.T("menu_title"))
}

// handleCallback маршрутизирует нажатия inline-кнопок.
func (h *Handler) handleCallback(ctx context.Context, cb *tgbotapi.CallbackQuery) {
	callback := tgbotapi.NewCallback(cb.ID, "")
	if _, err := h.bot.Request(callback); err != nil {
		h.log.Error("handleCallback: failed to answer callback", zap.Error(err))
	}

	// Bolus calculator prefix routing.
	if strings.HasPrefix(cb.Data, "bolus:") {
		switch cb.Data {
		case "bolus:start":
			h.handleBolusStart(ctx, cb)
		case "bolus:glucose:confirm":
			h.handleBolusGlucoseConfirm(cb)
		case "bolus:glucose:manual":
			h.handleBolusGlucoseManual(cb)
		case "bolus:details":
			h.handleBolusDetails(cb)
		case "bolus:save":
			h.handleBolusSave(ctx, cb)
		case "bolus:cancel":
			h.sessions.Delete(cb.Message.Chat.ID)
			h.handleMenuBack(ctx, cb)
		}
		return
	}

	// Advisor interval prefix routing.
	if strings.HasPrefix(cb.Data, "profile:advisor_interval:") {
		suffix := strings.TrimPrefix(cb.Data, "profile:advisor_interval:")
		switch suffix {
		case "3":
			h.handleAdvisorIntervalSet(ctx, cb, 3)
		case "7":
			h.handleAdvisorIntervalSet(ctx, cb, 7)
		case "14":
			h.handleAdvisorIntervalSet(ctx, cb, 14)
		case "off":
			h.handleAdvisorIntervalSet(ctx, cb, 0)
		case "custom":
			h.handleAdvisorIntervalCustom(cb)
		}
		return
	}

	if strings.HasPrefix(cb.Data, "profile:bolus_drug:set:") {
		drugKey := strings.TrimPrefix(cb.Data, "profile:bolus_drug:set:")
		h.handleBolusDrugSet(ctx, cb, drugKey)
		return
	}

	// Timezone selection prefix routing.
	if strings.HasPrefix(cb.Data, "profile:timezone:set:") {
		iana := strings.TrimPrefix(cb.Data, "profile:timezone:set:")
		h.handleTimezoneSet(ctx, cb, iana)
		return
	}

	// Note and diary prefix-based routing.
	if strings.HasPrefix(cb.Data, "note:") {
		h.handleNoteCallback(ctx, cb)
		return
	}
	if strings.HasPrefix(cb.Data, "diary:") {
		h.handleDiaryCallback(ctx, cb)
		return
	}

	// Activity prefix-based routing (before the main switch).
	if strings.HasPrefix(cb.Data, "activity:type:") {
		h.handleActivityTypeSelect(cb)
		return
	}
	if strings.HasPrefix(cb.Data, "activity:dur:") {
		h.handleActivityDurationQuick(ctx, cb)
		return
	}
	if cb.Data == "activity:time:manual" {
		h.handleActivityTimeManual(cb)
		return
	}
	if strings.HasPrefix(cb.Data, "activity:time:") {
		h.handleActivityTimeQuick(ctx, cb)
		return
	}
	if strings.HasPrefix(cb.Data, "activity:intensity:") {
		h.handleActivityIntensitySelect(cb)
		return
	}

	switch cb.Data {
	case "menu:back":
		h.sessions.Delete(cb.Message.Chat.ID)
		h.handleMenuBack(ctx, cb)
	case "menu:new_entry":
		h.handleNewEntryCb(cb)
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
	case "insulin:manual":
		sess := newSession(sessionInsulin, stepInsulinDose)
		sess.Data["type"] = string(domain.InsulinTypeBolus)
		h.sessions.Store(cb.Message.Chat.ID, sess)
		h.replyWithKeyboard(cb.Message.Chat.ID, h.loc.T("insulin_dose_prompt"), h.backToMenuKeyboard())
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
	case "menu:activity":
		h.handleActivityStart(cb)
	case "activity:history":
		h.handleActivityHistory(ctx, cb)
	case "menu:analytics":
		h.handleAnalytics(ctx, cb)
	case "menu:note":
		h.handleNoteStart(ctx, cb)
	case "menu:diary":
		h.handleDiaryShow(ctx, cb, time.Now())
	case "profile:target_range":
		h.handleProfileTargetRangeStart(cb)
	case "profile:bolus_drug":
		h.handleBolusDrugMenu(cb)
	case "profile:basal":
		h.handleProfileBasalStart(cb)
	case "profile:basal:skip_drug":
		h.handleProfileBasalSkipDrug(cb)
	case "profile:basal:skip_time":
		h.handleProfileBasalSkipTime(ctx, cb)
	case "profile:timezone":
		h.handleTimezoneMenu(cb)
	case "menu:advisor":
		h.handleAdvisorShow(ctx, cb)
	case "profile:basal_dose":
		h.handleProfileBasalDoseStart(cb)
	case "profile:advisor_interval":
		h.handleAdvisorIntervalMenu(cb)
	}
}

// handleMenuBack — возврат в главное меню.
func (h *Handler) handleMenuBack(_ context.Context, cb *tgbotapi.CallbackQuery) {
	h.sendMenu(cb.Message.Chat.ID, h.loc.T("menu_title"))
}

// handleNewEntryCb shows the "new entry" submenu.
func (h *Handler) handleNewEntryCb(cb *tgbotapi.CallbackQuery) {
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(h.loc.T("btn_glucose"), "menu:glucose"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(h.loc.T("btn_food"), "menu:food"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(h.loc.T("btn_insulin"), "menu:insulin"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(h.loc.T("btn_activity"), "menu:activity"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(h.loc.T("btn_note"), "menu:note"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(h.loc.T("btn_back_menu"), "menu:back"),
		),
	)
	h.replyWithKeyboard(cb.Message.Chat.ID, h.loc.T("menu_title"), keyboard)
}

// sendProfileView loads the user profile and sends the profile screen to chatID.
// telegramUserID is the Telegram user's numeric ID.
func (h *Handler) sendProfileView(ctx context.Context, chatID, telegramUserID int64) {
	user, acc, err := h.userUC.GetProfile(ctx, domain.ProviderTelegram, strconv.FormatInt(telegramUserID, 10))
	if err != nil || user == nil {
		h.log.Error("sendProfileView: failed to get profile", zap.Error(err))
		h.reply(chatID, h.loc.T("error_internal"))
		return
	}

	label := h.unitsLabel(string(user.Units))

	rangeStr := fmt.Sprintf("⚙️ Диапазон: %s–%s ммоль/л",
		formatMmol(user.TargetMinMmol),
		formatMmol(user.TargetMaxMmol),
	)

	var basalStr string
	if user.BasalDrug == "" && user.BasalTime == "" {
		basalStr = "💉 Базальный: не задан"
	} else {
		parts := user.BasalDrug
		if user.BasalTime != "" {
			if parts != "" {
				parts += " · " + user.BasalTime
			} else {
				parts = user.BasalTime
			}
		}
		basalStr = "💉 Базальный: " + parts
	}

	var bolusDrugStr string
	if user.BolusDrug == "" {
		bolusDrugStr = "💉 Болюсный: не задан"
	} else if profile, ok := domain.BolusInsulinCatalog[user.BolusDrug]; ok {
		bolusDrugStr = "💉 Болюсный: " + profile.Name
	} else {
		bolusDrugStr = "💉 Болюсный: " + user.BolusDrug
	}

	var basalDoseStr string
	if user.BasalDose == 0 {
		basalDoseStr = "💉 Доза базального: не задана"
	} else {
		basalDoseStr = "💉 Доза базального: " + formatDoseUnits(user.BasalDose) + " ед."
	}

	tz := user.Timezone
	if tz == "" {
		tz = "UTC"
	}

	text := h.loc.T("profile",
		acc.DisplayName,
		label,
		user.CreatedAt.Format("02.01.2006"),
	) + "\n" + rangeStr + "\n" + basalStr + "\n" + basalDoseStr + "\n" + bolusDrugStr + "\n" + fmt.Sprintf(h.loc.T("profile_timezone"), tz)

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(h.loc.T("btn_back_menu"), "menu:back"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(h.loc.T("profile_target_range"), "profile:target_range"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(h.loc.T("profile_basal"), "profile:basal"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(h.loc.T("profile_bolus_drug"), "profile:bolus_drug"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(h.loc.T("btn_units", label), "menu:units"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(h.loc.T("btn_carbs_unit", carbsPerUnitLabel(user.CarbsPerUnit)), "menu:carbs_unit"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(h.loc.T("profile_timezone_btn"), "profile:timezone"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(h.loc.T("profile_basal_dose"), "profile:basal_dose"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(h.loc.T("profile_advisor_interval"), "profile:advisor_interval"),
		),
	)

	h.replyWithKeyboard(chatID, text, keyboard)
}

// handleProfileCb — профиль по нажатию кнопки.
func (h *Handler) handleProfileCb(ctx context.Context, cb *tgbotapi.CallbackQuery) {
	h.sendProfileView(ctx, cb.Message.Chat.ID, cb.From.ID)
}

// handleSetUnitsCb — выбор единиц измерения по нажатию кнопки.
func (h *Handler) handleSetUnitsCb(_ context.Context, cb *tgbotapi.CallbackQuery) {
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(h.loc.T("units_mmol"), "units:mmol"),
			tgbotapi.NewInlineKeyboardButtonData(h.loc.T("units_mgdl"), "units:mgdl"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(h.loc.T("btn_back_menu"), "menu:profile"),
		),
	)

	h.replyWithKeyboard(cb.Message.Chat.ID, h.loc.T("setunits_prompt"), keyboard)
}

// setUserUnits обновляет единицы измерения и показывает профиль.
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

	h.sendProfileView(ctx, cb.Message.Chat.ID, cb.From.ID)
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
			tgbotapi.NewInlineKeyboardButtonData(h.loc.T("btn_back_menu"), "menu:profile"),
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

	h.sendProfileView(ctx, cb.Message.Chat.ID, cb.From.ID)
}

// handleCarbsUnitCustom starts a sessionCarbsUnit for custom input.
func (h *Handler) handleCarbsUnitCustom(cb *tgbotapi.CallbackQuery) {
	sess := newSession(sessionCarbsUnit, stepCarbsUnitValue)
	h.sessions.Store(cb.Message.Chat.ID, sess)
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(h.loc.T("btn_back_menu"), "menu:profile"),
		),
	)
	h.replyWithKeyboard(cb.Message.Chat.ID, h.loc.T("carbs_unit_custom_prompt"), keyboard)
}

// handleCarbsUnitStep processes text input during a sessionCarbsUnit flow.
func (h *Handler) handleCarbsUnitStep(ctx context.Context, msg *tgbotapi.Message, _ *Session) {
	h.sessions.Delete(msg.Chat.ID)

	grams, err := strconv.ParseFloat(strings.TrimSpace(msg.Text), 64)
	if err != nil {
		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData(h.loc.T("btn_back_menu"), "menu:profile"),
			),
		)
		h.replyWithKeyboard(msg.Chat.ID, h.loc.T("carbs_unit_invalid"), keyboard)
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

	h.sendProfileView(ctx, msg.Chat.ID, msg.From.ID)
}
