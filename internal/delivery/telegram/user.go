package telegram

import (
	"context"
	"strconv"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.uber.org/zap"

	"github.com/gmtantsevov/shuggabuddy/internal/domain"
)

// /start — приветствие + главное меню.
func (h *Handler) handleStart(ctx context.Context, msg *tgbotapi.Message) {
	h.waitingGlucose.Delete(msg.Chat.ID)

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

	label := h.unitsLabel(string(user.Units))

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

	h.sendMenu(msg.Chat.ID, greeting+"\n\n"+h.loc.T("menu_title"), label)
}

// handleCallback маршрутизирует нажатия inline-кнопок.
func (h *Handler) handleCallback(ctx context.Context, cb *tgbotapi.CallbackQuery) {
	callback := tgbotapi.NewCallback(cb.ID, "")
	if _, err := h.bot.Request(callback); err != nil {
		h.log.Error("handleCallback: failed to answer callback", zap.Error(err))
	}

	switch cb.Data {
	case "menu:back":
		h.waitingGlucose.Delete(cb.Message.Chat.ID)
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

	label := h.unitsLabel(string(user.Units))
	h.sendMenu(cb.Message.Chat.ID, h.loc.T("menu_title"), label)
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
	text := h.loc.T("setunits_success", label) + "\n\n" + h.loc.T("menu_title")
	h.sendMenu(cb.Message.Chat.ID, text, label)
}
