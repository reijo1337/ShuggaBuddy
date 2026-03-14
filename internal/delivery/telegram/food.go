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
	fooduc "github.com/gmtantsevov/shuggabuddy/internal/usecase/food"
)

// handleFoodStart begins the food entry flow.
func (h *Handler) handleFoodStart(ctx context.Context, cb *tgbotapi.CallbackQuery) {
	user, _, err := h.userUC.GetProfile(ctx, domain.ProviderTelegram, strconv.FormatInt(cb.From.ID, 10))
	if err != nil || user == nil {
		h.log.Error("handleFoodStart: failed to get user", zap.Error(err))
		h.reply(cb.Message.Chat.ID, h.loc.T("error_internal"))
		return
	}

	sess := newSession(sessionFood, stepFoodCarbs)
	sess.Data["carbs_unit"] = "g"
	h.sessions.Store(cb.Message.Chat.ID, sess)

	h.replyWithKeyboard(cb.Message.Chat.ID, h.loc.T("food_prompt_carbs"), h.carbsToggleKeyboard("g"))
}

// handleFoodUnitToggle handles food:unit:g / food:unit:xe callbacks.
func (h *Handler) handleFoodUnitToggle(_ context.Context, cb *tgbotapi.CallbackQuery) {
	raw, ok := h.sessions.Load(cb.Message.Chat.ID)
	if !ok {
		return
	}
	sess := raw.(*Session)
	if sess.SType != sessionFood || sess.Step != stepFoodCarbs {
		return
	}

	unit := "g"
	if cb.Data == "food:unit:xe" {
		unit = "xe"
	}
	sess.Data["carbs_unit"] = unit
	h.replyWithKeyboard(cb.Message.Chat.ID, h.loc.T("food_prompt_carbs"), h.carbsToggleKeyboard(unit))
}

// carbsToggleKeyboard builds the carbs input keyboard with the active unit toggled.
func (h *Handler) carbsToggleKeyboard(activeUnit string) tgbotapi.InlineKeyboardMarkup {
	gLabel := h.loc.T("food_toggle_g_inactive")
	xeLabel := h.loc.T("food_toggle_xe_inactive")
	if activeUnit == "g" {
		gLabel = h.loc.T("food_toggle_g")
	} else {
		xeLabel = h.loc.T("food_toggle_xe")
	}

	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(gLabel, "food:unit:g"),
			tgbotapi.NewInlineKeyboardButtonData(xeLabel, "food:unit:xe"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(h.loc.T("btn_back_menu"), "menu:back"),
		),
	)
}

// handleFoodStep routes food session text input to the correct step handler.
func (h *Handler) handleFoodStep(ctx context.Context, msg *tgbotapi.Message, sess *Session) {
	switch sess.Step {
	case stepFoodCarbs:
		h.handleFoodCarbsInput(ctx, msg, sess)
	case stepFoodNote:
		h.handleFoodNoteInput(msg, sess)
	case stepFoodTime:
		h.handleFoodTimeInput(ctx, msg, sess)
	}
}

// handleFoodCarbsInput processes the numeric carbs value.
func (h *Handler) handleFoodCarbsInput(ctx context.Context, msg *tgbotapi.Message, sess *Session) {
	text := strings.TrimSpace(msg.Text)
	value, err := strconv.ParseFloat(text, 64)
	if err != nil {
		h.replyWithKeyboard(msg.Chat.ID, h.loc.T("food_invalid_carbs"), h.carbsToggleKeyboard(sess.Data["carbs_unit"].(string)))
		return
	}

	unit := sess.Data["carbs_unit"].(string)
	carbsGrams := value

	if unit == "xe" {
		user, _, err := h.userUC.GetProfile(ctx, domain.ProviderTelegram, strconv.FormatInt(msg.From.ID, 10))
		if err != nil || user == nil {
			h.log.Error("handleFoodCarbsInput: failed to get user", zap.Error(err))
			h.reply(msg.Chat.ID, h.loc.T("error_internal"))
			return
		}
		carbsGrams = value * user.CarbsPerUnit
	}

	if carbsGrams < fooduc.MinCarbsGrams || carbsGrams > fooduc.MaxCarbsGrams {
		h.replyWithKeyboard(msg.Chat.ID, h.loc.T("food_out_of_range"), h.carbsToggleKeyboard(unit))
		return
	}

	sess.Data["carbs_grams"] = carbsGrams
	sess.Step = stepFoodNote
	h.sessions.Store(msg.Chat.ID, sess)

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(h.loc.T("food_btn_skip"), "food:skip_note"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(h.loc.T("btn_back_menu"), "menu:back"),
		),
	)
	h.replyWithKeyboard(msg.Chat.ID, h.loc.T("food_prompt_note"), keyboard)
}

// handleFoodSkipNote handles the food:skip_note callback.
func (h *Handler) handleFoodSkipNote(_ context.Context, cb *tgbotapi.CallbackQuery) {
	raw, ok := h.sessions.Load(cb.Message.Chat.ID)
	if !ok {
		return
	}
	sess := raw.(*Session)
	if sess.SType != sessionFood || sess.Step != stepFoodNote {
		return
	}
	sess.Data["note"] = ""
	sess.Step = stepFoodTime
	h.sessions.Store(cb.Message.Chat.ID, sess)
	h.replyWithKeyboard(cb.Message.Chat.ID, h.loc.T("food_prompt_time"), h.timeKeyboard())
}

// handleFoodNoteInput processes text note and advances to time step.
func (h *Handler) handleFoodNoteInput(msg *tgbotapi.Message, sess *Session) {
	sess.Data["note"] = strings.TrimSpace(msg.Text)
	sess.Step = stepFoodTime
	h.sessions.Store(msg.Chat.ID, sess)
	h.replyWithKeyboard(msg.Chat.ID, h.loc.T("food_prompt_time"), h.timeKeyboard())
}

// timeKeyboard builds the time selection keyboard.
func (h *Handler) timeKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(h.loc.T("food_btn_now"), "food:time:now"),
			tgbotapi.NewInlineKeyboardButtonData(h.loc.T("food_btn_minus15"), "food:time:-15"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(h.loc.T("food_btn_minus30"), "food:time:-30"),
			tgbotapi.NewInlineKeyboardButtonData(h.loc.T("food_btn_minus1h"), "food:time:-60"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(h.loc.T("food_btn_manual_time"), "food:time:manual"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(h.loc.T("btn_back_menu"), "menu:back"),
		),
	)
}

// handleFoodTimeQuick handles the preset time offset callbacks.
func (h *Handler) handleFoodTimeQuick(ctx context.Context, cb *tgbotapi.CallbackQuery) {
	raw, ok := h.sessions.Load(cb.Message.Chat.ID)
	if !ok {
		return
	}
	sess := raw.(*Session)
	if sess.SType != sessionFood || sess.Step != stepFoodTime {
		return
	}

	now := time.Now()
	var eatenAt time.Time
	switch cb.Data {
	case "food:time:now":
		eatenAt = now
	case "food:time:-15":
		eatenAt = now.Add(-15 * time.Minute)
	case "food:time:-30":
		eatenAt = now.Add(-30 * time.Minute)
	case "food:time:-60":
		eatenAt = now.Add(-60 * time.Minute)
	default:
		return
	}

	h.saveFoodEntry(ctx, cb.Message.Chat.ID, cb.From.ID, sess, eatenAt)
}

// handleFoodTimeManual sets the session to stepFoodTime and prompts for manual input.
func (h *Handler) handleFoodTimeManual(cb *tgbotapi.CallbackQuery) {
	raw, ok := h.sessions.Load(cb.Message.Chat.ID)
	if !ok {
		return
	}
	sess := raw.(*Session)
	if sess.SType != sessionFood || sess.Step != stepFoodTime {
		return
	}
	// Step is already stepFoodTime; send the format prompt.
	h.replyWithKeyboard(cb.Message.Chat.ID, h.loc.T("food_time_prompt"), h.backToMenuKeyboard())
}

// handleFoodTimeInput parses manual time text and saves the entry.
func (h *Handler) handleFoodTimeInput(ctx context.Context, msg *tgbotapi.Message, sess *Session) {
	eatenAt, err := parseTime(strings.TrimSpace(msg.Text))
	if err != nil {
		h.replyWithKeyboard(msg.Chat.ID, h.loc.T("food_time_invalid"), h.backToMenuKeyboard())
		return
	}
	h.saveFoodEntry(ctx, msg.Chat.ID, msg.From.ID, sess, eatenAt)
}

// parseTime tries to parse a user-supplied time string in three formats.
func parseTime(s string) (time.Time, error) {
	loc := time.Local
	now := time.Now().In(loc)

	// Format 1: HH:MM — today
	if t, err := time.ParseInLocation("15:04", s, loc); err == nil {
		return time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), 0, 0, loc), nil
	}

	// Format 2: DD.MM HH:MM — current year
	if t, err := time.ParseInLocation("02.01 15:04", s, loc); err == nil {
		return time.Date(now.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), 0, 0, loc), nil
	}

	// Format 3: DD.MM.YYYY HH:MM
	if t, err := time.ParseInLocation("02.01.2006 15:04", s, loc); err == nil {
		return t, nil
	}

	return time.Time{}, fmt.Errorf("parseTime: unrecognised format %q", s)
}

// saveFoodEntry persists the food entry and clears the session.
func (h *Handler) saveFoodEntry(ctx context.Context, chatID, fromID int64, sess *Session, eatenAt time.Time) {
	h.sessions.Delete(chatID)

	carbsGrams := sess.Data["carbs_grams"].(float64)
	note := sess.Data["note"].(string)

	user, _, err := h.userUC.GetProfile(ctx, domain.ProviderTelegram, strconv.FormatInt(fromID, 10))
	if err != nil || user == nil {
		h.log.Error("saveFoodEntry: failed to get user", zap.Error(err))
		h.reply(chatID, h.loc.T("error_internal"))
		return
	}

	if err := h.foodUC.SaveEntry(ctx, user.ID, carbsGrams, note, eatenAt); err != nil {
		h.log.Error("saveFoodEntry: failed to save entry", zap.Error(err))
		h.reply(chatID, h.loc.T("error_internal"))
		return
	}

	timeStr := eatenAt.Format("02.01 15:04")
	carbsStr := fooduc.FormatCarbs(carbsGrams)

	var text string
	if note != "" {
		text = h.loc.T("food_saved_with_note", carbsStr, timeStr, note)
	} else {
		text = h.loc.T("food_saved", carbsStr, timeStr)
	}

	h.replyWithKeyboard(chatID, text, h.backToMenuKeyboard())
}
