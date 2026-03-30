package telegram

import (
	"context"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.uber.org/zap"

	"github.com/gmtantsevov/shuggabuddy/internal/domain"
)

// handleNoteStart initiates the note recording flow by showing note type selection.
func (h *Handler) handleNoteStart(_ context.Context, cb *tgbotapi.CallbackQuery) {
	h.replyWithKeyboard(cb.Message.Chat.ID, h.loc.T("note_type_select"), h.noteTypeKeyboard())
}

// handleNoteCallback handles all note: prefix callbacks.
func (h *Handler) handleNoteCallback(ctx context.Context, cb *tgbotapi.CallbackQuery) {
	data := cb.Data

	switch {
	case strings.HasPrefix(data, "note:type:"):
		noteType := strings.TrimPrefix(data, "note:type:")
		h.handleNoteTypeSelect(ctx, cb, noteType)
	case strings.HasPrefix(data, "note:wellbeing:"):
		wellbeing := strings.TrimPrefix(data, "note:wellbeing:")
		h.handleNoteWellbeingSelect(cb, wellbeing)
	case data == "note:skip":
		h.handleNoteSkip(ctx, cb)
	}
}

// handleNoteTypeSelect handles selection of a note type.
func (h *Handler) handleNoteTypeSelect(_ context.Context, cb *tgbotapi.CallbackQuery, noteType string) {
	if noteType == string(domain.NoteTypeWellbeing) {
		// Show wellbeing sub-selection
		sess := newSession(sessionNote, stepNoteWellbeing)
		sess.Data["note_type"] = noteType
		h.sessions.Store(cb.Message.Chat.ID, sess)
		h.replyWithKeyboard(cb.Message.Chat.ID, h.loc.T("note_wellbeing_select"), h.noteWellbeingKeyboard())
		return
	}

	// For illness, stress, free: go directly to text prompt
	sess := newSession(sessionNote, stepNoteText)
	sess.Data["note_type"] = noteType
	h.sessions.Store(cb.Message.Chat.ID, sess)
	h.replyWithKeyboard(cb.Message.Chat.ID, h.loc.T("note_text_prompt"), h.noteSkipKeyboard())
}

// handleNoteWellbeingSelect handles selection of a wellbeing value.
func (h *Handler) handleNoteWellbeingSelect(cb *tgbotapi.CallbackQuery, wellbeing string) {
	sess := newSession(sessionNote, stepNoteText)
	sess.Data["note_type"] = string(domain.NoteTypeWellbeing)
	sess.Data["note_wellbeing"] = wellbeing
	h.sessions.Store(cb.Message.Chat.ID, sess)
	h.replyWithKeyboard(cb.Message.Chat.ID, h.loc.T("note_text_prompt"), h.noteSkipKeyboard())
}

// handleNoteSkip handles the note:skip callback — saves with nil text.
func (h *Handler) handleNoteSkip(ctx context.Context, cb *tgbotapi.CallbackQuery) {
	raw, ok := h.sessions.Load(cb.Message.Chat.ID)
	if !ok {
		return
	}
	sess := raw.(*Session)
	if sess.SType != sessionNote || sess.Step != stepNoteText {
		return
	}
	h.sessions.Delete(cb.Message.Chat.ID)

	noteType, wellbeing := h.extractNoteSessionData(sess)
	h.saveNote(ctx, cb.Message.Chat.ID, cb.From.ID, noteType, wellbeing, nil)
}

// handleNoteText handles text input during a note session.
func (h *Handler) handleNoteText(ctx context.Context, msg *tgbotapi.Message, sess *Session) {
	h.sessions.Delete(msg.Chat.ID)

	text := strings.TrimSpace(msg.Text)
	noteType, wellbeing := h.extractNoteSessionData(sess)
	h.saveNote(ctx, msg.Chat.ID, msg.From.ID, noteType, wellbeing, &text)
}

// extractNoteSessionData extracts note type and optional wellbeing from session data.
func (h *Handler) extractNoteSessionData(sess *Session) (domain.NoteType, *domain.WellbeingValue) {
	noteType := domain.NoteType(sess.Data["note_type"].(string))

	var wellbeing *domain.WellbeingValue
	if wb, ok := sess.Data["note_wellbeing"]; ok {
		wbVal := domain.WellbeingValue(wb.(string))
		wellbeing = &wbVal
	}

	return noteType, wellbeing
}

// saveNote calls the use case to persist the note and sends a confirmation message.
func (h *Handler) saveNote(ctx context.Context, chatID, fromID int64, noteType domain.NoteType, wellbeing *domain.WellbeingValue, text *string) {
	user, _, err := h.userUC.GetProfile(ctx, domain.ProviderTelegram, strconv.FormatInt(fromID, 10))
	if err != nil || user == nil {
		h.log.Error("saveNote: failed to get user", zap.Error(err))
		h.replyWithKeyboard(chatID, h.loc.T("error_internal"), h.backToMenuKeyboard())
		return
	}

	if err := h.noteUC.SaveNote(ctx, user.ID, noteType, wellbeing, text); err != nil {
		h.log.Error("saveNote: failed to save note", zap.Error(err))
		h.replyWithKeyboard(chatID, h.loc.T("error_internal"), h.backToMenuKeyboard())
		return
	}
	h.replyWithKeyboard(chatID, h.loc.T("note_saved"), h.backToMenuKeyboard())
}

// noteTypeKeyboard builds the note type selection keyboard.
func (h *Handler) noteTypeKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(h.loc.T("note_type_wellbeing"), "note:type:wellbeing"),
			tgbotapi.NewInlineKeyboardButtonData(h.loc.T("note_type_illness"), "note:type:illness"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(h.loc.T("note_type_stress"), "note:type:stress"),
			tgbotapi.NewInlineKeyboardButtonData(h.loc.T("note_type_free"), "note:type:free"),
		),
	)
}

// noteWellbeingKeyboard builds the wellbeing value selection keyboard.
func (h *Handler) noteWellbeingKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(h.loc.T("note_wellbeing_good"), "note:wellbeing:good"),
			tgbotapi.NewInlineKeyboardButtonData(h.loc.T("note_wellbeing_normal"), "note:wellbeing:normal"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(h.loc.T("note_wellbeing_bad"), "note:wellbeing:bad"),
			tgbotapi.NewInlineKeyboardButtonData(h.loc.T("note_wellbeing_sick"), "note:wellbeing:sick"),
		),
	)
}

// noteSkipKeyboard builds the keyboard with a skip button for text input.
func (h *Handler) noteSkipKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(h.loc.T("note_skip"), "note:skip"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(h.loc.T("btn_back_menu"), "menu:back"),
		),
	)
}
