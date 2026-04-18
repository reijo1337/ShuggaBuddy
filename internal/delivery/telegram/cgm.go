package telegram

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.uber.org/zap"

	"github.com/gmtantsevov/shuggabuddy/internal/domain"
	"github.com/gmtantsevov/shuggabuddy/pkg/nightscout"
)

// cgmUserID resolves internal userID from Telegram user via GetProfile.
func (h *Handler) cgmUserID(ctx context.Context, chatID, telegramUserID int64) (int64, bool) {
	user, _, err := h.userUC.GetProfile(ctx, domain.ProviderTelegram, strconv.FormatInt(telegramUserID, 10))
	if err != nil || user == nil {
		h.log.Error("cgm: failed to resolve user", zap.Error(err))
		h.reply(chatID, h.loc.T("error_internal"))
		return 0, false
	}
	return user.ID, true
}

// handleCGMCallback routes cgm: prefixed callbacks.
func (h *Handler) handleCGMCallback(ctx context.Context, cb *tgbotapi.CallbackQuery) {
	chatID := cb.Message.Chat.ID

	userID, ok := h.cgmUserID(ctx, chatID, cb.From.ID)
	if !ok {
		return
	}

	switch cb.Data {
	case "profile:cgm":
		h.handleCGMShow(ctx, chatID, userID)
	case "cgm:connect":
		h.handleCGMConnectStart(chatID)
	case "cgm:test":
		h.handleCGMTest(ctx, chatID, userID)
	case "cgm:disconnect":
		h.handleCGMDisconnectConfirm(chatID)
	case "cgm:disconnect:yes":
		h.handleCGMDisconnect(ctx, chatID, userID)
	case "cgm:disconnect:no", "cgm:back":
		h.handleCGMShow(ctx, chatID, userID)
	}
}

func (h *Handler) handleCGMShow(ctx context.Context, chatID, userID int64) {
	h.sessions.Delete(chatID)

	if h.cgmUC == nil {
		h.reply(chatID, h.loc.T("cgm_no_encryption_key"))
		return
	}

	conn, err := h.cgmUC.GetConnection(ctx, userID)
	if err != nil {
		h.log.Error("cgm: failed to get connection", zap.Error(err))
		h.reply(chatID, h.loc.T("error_internal"))
		return
	}

	if conn != nil && conn.Active {
		lastSync := "—"
		if conn.LastSyncedAt != nil {
			lastSync = conn.LastSyncedAt.Format("02.01 15:04")
		}

		text := fmt.Sprintf(h.loc.T("cgm_status_connected"), conn.BaseURL, lastSync)
		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData(h.loc.T("cgm_btn_test"), "cgm:test"),
			),
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData(h.loc.T("cgm_btn_disconnect"), "cgm:disconnect"),
			),
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData(h.loc.T("btn_back_menu"), "menu:back"),
			),
		)
		h.replyWithKeyboard(chatID, text, keyboard)
		return
	}

	// Not connected — show intro with instructions.
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(h.loc.T("cgm_btn_connect"), "cgm:connect"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(h.loc.T("cgm_btn_back"), "menu:profile"),
		),
	)
	h.replyWithKeyboard(chatID, h.loc.T("cgm_intro"), keyboard)
}

func (h *Handler) handleCGMConnectStart(chatID int64) {
	sess := newSession(sessionCGM, stepCGMURL)
	h.sessions.Store(chatID, sess)
	h.replyWithKeyboard(chatID, h.loc.T("cgm_url_prompt"), h.backToMenuKeyboard())
}

func (h *Handler) handleCGMStep(ctx context.Context, msg *tgbotapi.Message, sess *Session) {
	switch sess.Step {
	case stepCGMURL:
		h.handleCGMURLInput(msg, sess)
	case stepCGMToken:
		h.handleCGMTokenInput(ctx, msg, sess)
	}
}

func (h *Handler) handleCGMURLInput(msg *tgbotapi.Message, sess *Session) {
	url := strings.TrimSpace(msg.Text)

	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		h.reply(msg.Chat.ID, h.loc.T("cgm_url_invalid"))
		return
	}

	sess.Data["url"] = url
	sess.Step = stepCGMToken
	h.replyWithKeyboard(msg.Chat.ID, h.loc.T("cgm_token_prompt"), h.backToMenuKeyboard())
}

func (h *Handler) handleCGMTokenInput(ctx context.Context, msg *tgbotapi.Message, sess *Session) {
	chatID := msg.Chat.ID
	token := strings.TrimSpace(msg.Text)
	url, _ := sess.Data["url"].(string)

	userID, ok := h.cgmUserID(ctx, chatID, msg.From.ID)
	if !ok {
		return
	}

	h.sessions.Delete(chatID)

	err := h.cgmUC.AddConnection(ctx, userID, url, token)
	if err != nil {
		h.log.Error("cgm: add connection failed", zap.Error(err))

		switch {
		case errors.Is(err, nightscout.ErrUnauthorized):
			h.reply(chatID, h.loc.T("cgm_auth_fail"))
		case errors.Is(err, nightscout.ErrNotFound):
			h.reply(chatID, h.loc.T("cgm_error_unreachable"))
		case strings.Contains(err.Error(), "HTTPS"):
			h.reply(chatID, h.loc.T("cgm_url_invalid"))
		default:
			h.reply(chatID, h.loc.T("cgm_test_fail"))
		}
		return
	}

	h.replyWithKeyboard(chatID, h.loc.T("cgm_connected"), h.backToMenuKeyboard())
}

func (h *Handler) handleCGMTest(ctx context.Context, chatID, userID int64) {
	err := h.cgmUC.TestConnection(ctx, userID)
	if err != nil {
		h.log.Error("cgm: test failed", zap.Error(err))
		if errors.Is(err, nightscout.ErrUnauthorized) {
			h.reply(chatID, h.loc.T("cgm_auth_fail"))
		} else {
			h.reply(chatID, h.loc.T("cgm_test_fail"))
		}
		return
	}
	h.reply(chatID, h.loc.T("cgm_test_ok"))
}

func (h *Handler) handleCGMDisconnectConfirm(chatID int64) {
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(h.loc.T("cgm_confirm_yes"), "cgm:disconnect:yes"),
			tgbotapi.NewInlineKeyboardButtonData(h.loc.T("cgm_confirm_no"), "cgm:disconnect:no"),
		),
	)
	h.replyWithKeyboard(chatID, h.loc.T("cgm_confirm_disconnect"), keyboard)
}

func (h *Handler) handleCGMDisconnect(ctx context.Context, chatID, userID int64) {
	if err := h.cgmUC.RemoveConnection(ctx, userID); err != nil {
		h.log.Error("cgm: disconnect failed", zap.Error(err))
		h.reply(chatID, h.loc.T("error_internal"))
		return
	}
	h.replyWithKeyboard(chatID, h.loc.T("cgm_disconnected"), h.backToMenuKeyboard())
}
