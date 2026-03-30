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
	"github.com/gmtantsevov/shuggabuddy/internal/i18n"
)

// handleDiaryShow shows diary entries for the given date.
// Called from menu:diary (today) or diary:show:YYYY-MM-DD navigation.
func (h *Handler) handleDiaryShow(ctx context.Context, cb *tgbotapi.CallbackQuery, date time.Time) {
	user, _, err := h.userUC.GetProfile(ctx, domain.ProviderTelegram, strconv.FormatInt(cb.From.ID, 10))
	if err != nil || user == nil {
		h.log.Error("handleDiaryShow: failed to get user", zap.Error(err))
		h.replyWithKeyboard(cb.Message.Chat.ID, h.loc.T("error_internal"), h.backToMenuKeyboard())
		return
	}

	loc := userLocation(user.Timezone)
	h.renderDiary(ctx, cb.Message.Chat.ID, cb.Message.MessageID, user.ID, date.In(loc), user, false)
}

// handleDiaryCallback handles diary: prefix callbacks.
func (h *Handler) handleDiaryCallback(ctx context.Context, cb *tgbotapi.CallbackQuery) {
	data := cb.Data

	switch {
	case strings.HasPrefix(data, "diary:show:"):
		dateStr := strings.TrimPrefix(data, "diary:show:")
		date, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			h.log.Error("handleDiaryCallback: invalid date in callback", zap.String("data", data), zap.Error(err))
			h.replyWithKeyboard(cb.Message.Chat.ID, h.loc.T("error_internal"), h.backToMenuKeyboard())
			return
		}

		user, _, err := h.userUC.GetProfile(ctx, domain.ProviderTelegram, strconv.FormatInt(cb.From.ID, 10))
		if err != nil || user == nil {
			h.log.Error("handleDiaryCallback: failed to get user", zap.Error(err))
			h.replyWithKeyboard(cb.Message.Chat.ID, h.loc.T("error_internal"), h.backToMenuKeyboard())
			return
		}

		loc := userLocation(user.Timezone)
		h.renderDiary(ctx, cb.Message.Chat.ID, cb.Message.MessageID, user.ID, date.In(loc), user, true)

	case data == "diary:date":
		sess := newSession(sessionDiary, stepDiaryDate)
		h.sessions.Store(cb.Message.Chat.ID, sess)
		h.replyWithKeyboard(cb.Message.Chat.ID, h.loc.T("diary_date_prompt"), h.backToMenuKeyboard())
	}
}

// handleDiaryText handles text input during a diary session.
func (h *Handler) handleDiaryText(ctx context.Context, msg *tgbotapi.Message, sess *Session) {
	if sess.Step != stepDiaryDate {
		return
	}

	text := strings.TrimSpace(msg.Text)
	date, err := parseDiaryDate(text)
	if err != nil {
		h.replyWithKeyboard(msg.Chat.ID, h.loc.T("diary_date_invalid"), h.backToMenuKeyboard())
		return
	}

	h.sessions.Delete(msg.Chat.ID)

	user, _, err := h.userUC.GetProfile(ctx, domain.ProviderTelegram, strconv.FormatInt(msg.From.ID, 10))
	if err != nil || user == nil {
		h.log.Error("handleDiaryText: failed to get user", zap.Error(err))
		h.replyWithKeyboard(msg.Chat.ID, h.loc.T("error_internal"), h.backToMenuKeyboard())
		return
	}

	loc := userLocation(user.Timezone)
	h.renderDiary(ctx, msg.Chat.ID, 0, user.ID, date.In(loc), user, false)
}

// renderDiary fetches entries for the given date and sends (or edits) the diary message.
// date must already be in the user's timezone location.
func (h *Handler) renderDiary(ctx context.Context, chatID int64, messageID int, userID int64, date time.Time, user *domain.User, edit bool) {
	loc := date.Location()
	entries, err := h.diaryUC.GetDayEntries(ctx, userID, date, loc)
	if err != nil {
		h.log.Error("renderDiary: failed to get diary entries", zap.Error(err))
		h.replyWithKeyboard(chatID, h.loc.T("error_internal"), h.backToMenuKeyboard())
		return
	}

	today := time.Now().In(loc)
	text := buildDiaryText(entries, date, user, h.loc, loc)
	keyboard := diaryNavKeyboard(date, today)

	if edit && messageID != 0 {
		editMsg := tgbotapi.NewEditMessageText(chatID, messageID, text)
		editMsg.ParseMode = "Markdown"
		editMsg.ReplyMarkup = &keyboard
		if _, err := h.bot.Request(editMsg); err != nil {
			h.log.Error("renderDiary: failed to edit message", zap.Error(err))
		}
		return
	}

	h.replyWithKeyboard(chatID, text, keyboard)
}

// buildDiaryText builds the full diary message text for a given date.
func buildDiaryText(entries []*domain.DiaryEntry, date time.Time, user *domain.User, loc *i18n.Localizer, tz *time.Location) string {
	title := fmt.Sprintf(loc.T("diary_title"), date.Format("02.01.2006"))

	if len(entries) == 0 {
		return title + "\n\n" + loc.T("diary_empty")
	}

	var sb strings.Builder
	sb.WriteString(title)
	sb.WriteString("\n\n")

	for _, entry := range entries {
		row := formatDiaryRow(entry, user, tz)
		if row != "" {
			sb.WriteString(row)
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// formatDiaryRow formats a single diary entry as "HH:MM  <icon> <value>[ indicator]".
func formatDiaryRow(entry *domain.DiaryEntry, user *domain.User, tz *time.Location) string {
	timeStr := entry.Time.In(tz).Format("15:04")

	switch entry.Kind {
	case domain.DiaryKindGlucose:
		if entry.Glucose == nil {
			return ""
		}
		valueStr := strconv.FormatFloat(entry.Glucose.ValueMmol, 'f', 1, 64)
		indicator := glucoseStatusEmoji(entry.Glucose.ValueMmol, user.TargetMinMmol, user.TargetMaxMmol)
		return fmt.Sprintf("%s  🩸 %s ммоль/л %s", timeStr, valueStr, indicator)

	case domain.DiaryKindFood:
		if entry.Food == nil {
			return ""
		}
		carbsStr := strconv.FormatFloat(entry.Food.CarbsGrams, 'f', -1, 64)
		return fmt.Sprintf("%s  🍽 %sг", timeStr, carbsStr)

	case domain.DiaryKindInsulin:
		if entry.Insulin == nil {
			return ""
		}
		doseStr := formatDoseUnits(entry.Insulin.DoseUnits)
		typeLabel := insulinTypeLabel(entry.Insulin.InsulinType)
		return fmt.Sprintf("%s  💉 %s ед. (%s)", timeStr, doseStr, typeLabel)

	case domain.DiaryKindActivity:
		if entry.Activity == nil {
			return ""
		}
		typeLabel := activityTypeDisplayLabel(entry.Activity.ActivityType, entry.Activity.CustomType)
		return fmt.Sprintf("%s  🏃 %s · %d мин", timeStr, typeLabel, entry.Activity.DurationMin)

	case domain.DiaryKindNote:
		if entry.Note == nil {
			return ""
		}
		if entry.Note.Wellbeing != nil {
			wellbeingLabel := wellbeingDisplayLabel(*entry.Note.Wellbeing)
			return fmt.Sprintf("%s  📝 %s", timeStr, wellbeingLabel)
		}
		text := "—"
		if entry.Note.Text != nil && *entry.Note.Text != "" {
			text = *entry.Note.Text
		}
		return fmt.Sprintf("%s  📝 %s", timeStr, text)
	}

	return ""
}

// diaryNavKeyboard builds the navigation keyboard for the diary.
// If showing today, the "Вперёд →" button is hidden.
func diaryNavKeyboard(date, today time.Time) tgbotapi.InlineKeyboardMarkup {
	prevDate := date.AddDate(0, 0, -1)
	nextDate := date.AddDate(0, 0, 1)

	prevBtn := tgbotapi.NewInlineKeyboardButtonData("← Назад", "diary:show:"+prevDate.Format("2006-01-02"))
	dateBtn := tgbotapi.NewInlineKeyboardButtonData("Выбрать дату", "diary:date")
	nextBtn := tgbotapi.NewInlineKeyboardButtonData("Вперёд →", "diary:show:"+nextDate.Format("2006-01-02"))

	if isSameDay(date, today) {
		return tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(prevBtn, dateBtn),
		)
	}

	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(prevBtn, dateBtn, nextBtn),
	)
}

// isSameDay reports whether a and b represent the same calendar day.
func isSameDay(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}

// parseDiaryDate parses a date string in "DD.MM" or "DD.MM.YYYY" format.
// The returned time has no location set (UTC zero); callers should In(userLoc) it.
func parseDiaryDate(s string) (time.Time, error) {
	// Try DD.MM.YYYY first
	if t, err := time.Parse("02.01.2006", s); err == nil {
		return t, nil
	}

	// Try DD.MM — use current year
	if t, err := time.Parse("02.01", s); err == nil {
		now := time.Now()
		return time.Date(now.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC), nil
	}

	return time.Time{}, fmt.Errorf("cannot parse date: %q", s)
}

// userLocation loads the IANA timezone for a user, falling back to UTC on error.
func userLocation(timezone string) *time.Location {
	if timezone == "" {
		return time.UTC
	}
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return time.UTC
	}
	return loc
}

// insulinTypeLabel returns localized insulin type label.
func insulinTypeLabel(t domain.InsulinType) string {
	switch t {
	case domain.InsulinTypeBolus:
		return "болюс"
	case domain.InsulinTypeBasal:
		return "базовый"
	default:
		return string(t)
	}
}

// activityTypeDisplayLabel returns human-readable label for activity type.
func activityTypeDisplayLabel(t domain.ActivityType, customType string) string {
	switch t {
	case domain.ActivityWalking:
		return "Ходьба"
	case domain.ActivityRunning:
		return "Бег"
	case domain.ActivityCycling:
		return "Велосипед"
	case domain.ActivityStrength:
		return "Силовая"
	case domain.ActivitySwimming:
		return "Плавание"
	case domain.ActivityYoga:
		return "Йога"
	case domain.ActivityDancing:
		return "Танцы"
	case domain.ActivityTeamSport:
		return "Команд. спорт"
	case domain.ActivitySkiing:
		return "Лыжи/сноуборд"
	case domain.ActivityOther:
		if customType != "" {
			return customType
		}
		return "Другое"
	default:
		if customType != "" {
			return customType
		}
		return string(t)
	}
}

// wellbeingDisplayLabel returns an emoji+label for wellbeing value.
func wellbeingDisplayLabel(w domain.WellbeingValue) string {
	switch w {
	case domain.WellbeingGood:
		return "😊 Хорошо"
	case domain.WellbeingNormal:
		return "😐 Нормально"
	case domain.WellbeingBad:
		return "😔 Плохо"
	case domain.WellbeingSick:
		return "🤒 Болею"
	default:
		return string(w)
	}
}
