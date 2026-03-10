package telegram

import (
	"context"
	"strconv"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.uber.org/zap"

	"github.com/gmtantsevov/shuggabuddy/internal/domain"
)

// /start.
func (h *Handler) handleStart(ctx context.Context, msg *tgbotapi.Message) {
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

	if created {
		h.log.Info("new user registered",
			zap.Int64("user_id", user.ID),
			zap.String("provider", string(domain.ProviderTelegram)),
		)
		h.reply(msg.Chat.ID, h.loc.T("welcome", acc.DisplayName))
	} else {
		h.reply(msg.Chat.ID, h.loc.T("welcome_back", acc.DisplayName))
	}
}

// /help.
func (h *Handler) handleHelp(msg *tgbotapi.Message) {
	h.reply(msg.Chat.ID, h.loc.T("help"))
}

// /profile.
func (h *Handler) handleProfile(ctx context.Context, msg *tgbotapi.Message) {
	user, acc, err := h.userUC.GetProfile(ctx, domain.ProviderTelegram, strconv.FormatInt(msg.From.ID, 10))
	if err != nil {
		h.log.Error("handleProfile: failed to get profile", zap.Error(err))
		h.reply(msg.Chat.ID, h.loc.T("error_internal"))
		return
	}

	if user == nil {
		h.reply(msg.Chat.ID, h.loc.T("welcome", msg.From.FirstName))
		return
	}

	unitsLabel := h.loc.T("units_mmol")
	if user.Units == domain.UnitsMgdl {
		unitsLabel = h.loc.T("units_mgdl")
	}

	h.reply(msg.Chat.ID, h.loc.T("profile",
		acc.DisplayName,
		unitsLabel,
		user.CreatedAt.Format("02.01.2006"),
	))
}

// /setunits — показывает inline-кнопки.
func (h *Handler) handleSetUnits(msg *tgbotapi.Message) {
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(h.loc.T("units_mmol"), "units:mmol"),
			tgbotapi.NewInlineKeyboardButtonData(h.loc.T("units_mgdl"), "units:mgdl"),
		),
	)

	h.replyWithKeyboard(msg.Chat.ID, h.loc.T("setunits_prompt"), keyboard)
}

// нажатия на inline-кнопки.
func (h *Handler) handleCallback(ctx context.Context, cb *tgbotapi.CallbackQuery) {
	callback := tgbotapi.NewCallback(cb.ID, "")
	if _, err := h.bot.Request(callback); err != nil {
		h.log.Error("handleCallback: failed to answer callback", zap.Error(err))
	}

	switch cb.Data {
	case "units:mmol":
		h.setUserUnits(ctx, cb, domain.UnitsMmol)
	case "units:mgdl":
		h.setUserUnits(ctx, cb, domain.UnitsMgdl)
	}
}

// обновляет единицы измерения и уведомляет пользователя.
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

	label := h.loc.T("units_mmol")
	if units == domain.UnitsMgdl {
		label = h.loc.T("units_mgdl")
	}

	h.reply(cb.Message.Chat.ID, h.loc.T("setunits_success", label))
}
