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

// activityTypeFromCallback extracts the activity type from callback data like "activity:type:running".
func activityTypeFromCallback(data string) (domain.ActivityType, bool) {
	parts := strings.SplitN(data, ":", 3)
	if len(parts) != 3 || parts[0] != "activity" || parts[1] != "type" {
		return "", false
	}
	at := domain.ActivityType(parts[2])
	return at, true
}

// handleActivityStart начинает флоу записи активности.
func (h *Handler) handleActivityStart(cb *tgbotapi.CallbackQuery) {
	sess := newSession(sessionActivity, stepActivityType)
	h.sessions.Store(cb.Message.Chat.ID, sess)

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(h.loc.T("activity_type_walking"), "activity:type:walking"),
			tgbotapi.NewInlineKeyboardButtonData(h.loc.T("activity_type_running"), "activity:type:running"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(h.loc.T("activity_type_cycling"), "activity:type:cycling"),
			tgbotapi.NewInlineKeyboardButtonData(h.loc.T("activity_type_strength"), "activity:type:strength"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(h.loc.T("activity_type_swimming"), "activity:type:swimming"),
			tgbotapi.NewInlineKeyboardButtonData(h.loc.T("activity_type_yoga"), "activity:type:yoga"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(h.loc.T("activity_type_dancing"), "activity:type:dancing"),
			tgbotapi.NewInlineKeyboardButtonData(h.loc.T("activity_type_team_sport"), "activity:type:team_sport"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(h.loc.T("activity_type_skiing"), "activity:type:skiing"),
			tgbotapi.NewInlineKeyboardButtonData(h.loc.T("activity_type_other"), "activity:type:other"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(h.loc.T("btn_back_menu"), "menu:back"),
		),
	)
	h.replyWithKeyboard(cb.Message.Chat.ID, h.loc.T("activity_type_prompt"), keyboard)
}

// handleActivityTypeSelect обрабатывает выбор типа активности.
func (h *Handler) handleActivityTypeSelect(cb *tgbotapi.CallbackQuery) {
	at, ok := activityTypeFromCallback(cb.Data)
	if !ok {
		return
	}

	raw, ok := h.sessions.Load(cb.Message.Chat.ID)
	if !ok {
		return
	}
	sess := raw.(*Session)
	if sess.SType != sessionActivity || sess.Step != stepActivityType {
		return
	}

	sess.Data["type"] = string(at)

	if at == domain.ActivityOther {
		sess.Step = stepActivityCustom
		h.sessions.Store(cb.Message.Chat.ID, sess)
		h.replyWithKeyboard(cb.Message.Chat.ID, h.loc.T("activity_custom_prompt"), h.backToMenuKeyboard())
		return
	}

	sess.Step = stepActivityDuration
	h.sessions.Store(cb.Message.Chat.ID, sess)
	h.replyWithKeyboard(cb.Message.Chat.ID, h.loc.T("activity_duration_prompt"), h.activityDurationKeyboard())
}

// handleActivityStep роутит текстовый ввод в зависимости от шага сессии.
func (h *Handler) handleActivityStep(ctx context.Context, msg *tgbotapi.Message, sess *Session) {
	switch sess.Step {
	case stepActivityCustom:
		h.handleActivityCustomInput(msg, sess)
	case stepActivityDuration:
		h.handleActivityDurationInput(ctx, msg, sess)
	case stepActivityTime:
		h.handleActivityTimeInput(ctx, msg, sess)
	}
}

// handleActivityCustomInput обрабатывает ввод произвольного типа активности.
func (h *Handler) handleActivityCustomInput(msg *tgbotapi.Message, sess *Session) {
	custom := strings.TrimSpace(msg.Text)
	if custom == "" || len(custom) > 100 {
		h.replyWithKeyboard(msg.Chat.ID, h.loc.T("activity_invalid_custom"), h.backToMenuKeyboard())
		return
	}
	sess.Data["custom_type"] = custom
	sess.Step = stepActivityDuration
	h.sessions.Store(msg.Chat.ID, sess)
	h.replyWithKeyboard(msg.Chat.ID, h.loc.T("activity_duration_prompt"), h.activityDurationKeyboard())
}

// activityDurationKeyboard builds the duration selection keyboard.
func (h *Handler) activityDurationKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(h.loc.T("activity_dur_15"), "activity:dur:15"),
			tgbotapi.NewInlineKeyboardButtonData(h.loc.T("activity_dur_30"), "activity:dur:30"),
			tgbotapi.NewInlineKeyboardButtonData(h.loc.T("activity_dur_45"), "activity:dur:45"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(h.loc.T("activity_dur_60"), "activity:dur:60"),
			tgbotapi.NewInlineKeyboardButtonData(h.loc.T("activity_dur_90"), "activity:dur:90"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(h.loc.T("btn_back_menu"), "menu:back"),
		),
	)
}

// handleActivityDurationQuick обрабатывает быстрые кнопки длительности.
func (h *Handler) handleActivityDurationQuick(_ context.Context, cb *tgbotapi.CallbackQuery) {
	raw, ok := h.sessions.Load(cb.Message.Chat.ID)
	if !ok {
		return
	}
	sess := raw.(*Session)
	if sess.SType != sessionActivity || sess.Step != stepActivityDuration {
		return
	}

	// Parse "activity:dur:30" → 30
	parts := strings.SplitN(cb.Data, ":", 3)
	if len(parts) != 3 {
		return
	}
	dur, err := strconv.Atoi(parts[2])
	if err != nil {
		return
	}

	sess.Data["duration"] = dur
	sess.Step = stepActivityIntensity
	h.sessions.Store(cb.Message.Chat.ID, sess)
	h.replyWithKeyboard(cb.Message.Chat.ID, h.loc.T("activity_intensity_prompt"), h.activityIntensityKeyboard())
}

// handleActivityDurationInput обрабатывает ручной ввод длительности.
func (h *Handler) handleActivityDurationInput(_ context.Context, msg *tgbotapi.Message, sess *Session) {
	text := strings.TrimSpace(msg.Text)
	dur, err := strconv.Atoi(text)
	if err != nil || dur < 1 || dur > 600 {
		h.replyWithKeyboard(msg.Chat.ID, h.loc.T("activity_invalid_duration"), h.activityDurationKeyboard())
		return
	}

	sess.Data["duration"] = dur
	sess.Step = stepActivityIntensity
	h.sessions.Store(msg.Chat.ID, sess)
	h.replyWithKeyboard(msg.Chat.ID, h.loc.T("activity_intensity_prompt"), h.activityIntensityKeyboard())
}

// activityIntensityKeyboard builds the intensity selection keyboard.
func (h *Handler) activityIntensityKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(h.loc.T("activity_intensity_low"), "activity:intensity:low"),
			tgbotapi.NewInlineKeyboardButtonData(h.loc.T("activity_intensity_medium"), "activity:intensity:medium"),
			tgbotapi.NewInlineKeyboardButtonData(h.loc.T("activity_intensity_high"), "activity:intensity:high"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(h.loc.T("activity_btn_skip_intensity"), "activity:intensity:medium"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(h.loc.T("btn_back_menu"), "menu:back"),
		),
	)
}

// handleActivityIntensitySelect обрабатывает выбор интенсивности.
func (h *Handler) handleActivityIntensitySelect(cb *tgbotapi.CallbackQuery) {
	raw, ok := h.sessions.Load(cb.Message.Chat.ID)
	if !ok {
		return
	}
	sess := raw.(*Session)
	if sess.SType != sessionActivity || sess.Step != stepActivityIntensity {
		return
	}

	parts := strings.SplitN(cb.Data, ":", 3)
	if len(parts) != 3 {
		return
	}
	intensity := domain.Intensity(parts[2])

	sess.Data["intensity"] = string(intensity)
	sess.Step = stepActivityTime
	h.sessions.Store(cb.Message.Chat.ID, sess)
	h.replyWithKeyboard(cb.Message.Chat.ID, h.loc.T("activity_time_prompt"), h.activityTimeKeyboard())
}

// activityTimeKeyboard builds the time selection keyboard (reuses food pattern).
func (h *Handler) activityTimeKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(h.loc.T("food_btn_now"), "activity:time:now"),
			tgbotapi.NewInlineKeyboardButtonData(h.loc.T("food_btn_minus15"), "activity:time:-15"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(h.loc.T("food_btn_minus30"), "activity:time:-30"),
			tgbotapi.NewInlineKeyboardButtonData(h.loc.T("food_btn_minus1h"), "activity:time:-60"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(h.loc.T("food_btn_manual_time"), "activity:time:manual"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(h.loc.T("btn_back_menu"), "menu:back"),
		),
	)
}

// handleActivityTimeQuick обрабатывает быстрые кнопки времени.
func (h *Handler) handleActivityTimeQuick(ctx context.Context, cb *tgbotapi.CallbackQuery) {
	raw, ok := h.sessions.Load(cb.Message.Chat.ID)
	if !ok {
		return
	}
	sess := raw.(*Session)
	if sess.SType != sessionActivity || sess.Step != stepActivityTime {
		return
	}

	now := time.Now()
	var recordedAt time.Time
	switch cb.Data {
	case "activity:time:now":
		recordedAt = now
	case "activity:time:-15":
		recordedAt = now.Add(-15 * time.Minute)
	case "activity:time:-30":
		recordedAt = now.Add(-30 * time.Minute)
	case "activity:time:-60":
		recordedAt = now.Add(-60 * time.Minute)
	default:
		return
	}

	h.saveActivityEntry(ctx, cb.Message.Chat.ID, cb.From.ID, sess, recordedAt)
}

// handleActivityTimeManual запрашивает ручной ввод времени.
func (h *Handler) handleActivityTimeManual(cb *tgbotapi.CallbackQuery) {
	raw, ok := h.sessions.Load(cb.Message.Chat.ID)
	if !ok {
		return
	}
	sess := raw.(*Session)
	if sess.SType != sessionActivity || sess.Step != stepActivityTime {
		return
	}
	h.replyWithKeyboard(cb.Message.Chat.ID, h.loc.T("activity_time_format"), h.backToMenuKeyboard())
}

// handleActivityTimeInput обрабатывает ручной ввод времени.
func (h *Handler) handleActivityTimeInput(ctx context.Context, msg *tgbotapi.Message, sess *Session) {
	recordedAt, err := parseTime(strings.TrimSpace(msg.Text))
	if err != nil {
		h.replyWithKeyboard(msg.Chat.ID, h.loc.T("activity_time_invalid"), h.backToMenuKeyboard())
		return
	}
	h.saveActivityEntry(ctx, msg.Chat.ID, msg.From.ID, sess, recordedAt)
}

// saveActivityEntry сохраняет запись и показывает подтверждение с оценкой.
func (h *Handler) saveActivityEntry(ctx context.Context, chatID, fromID int64, sess *Session, recordedAt time.Time) {
	h.sessions.Delete(chatID)

	activityType := domain.ActivityType(sess.Data["type"].(string))
	customType := ""
	if v, ok := sess.Data["custom_type"]; ok {
		customType = v.(string)
	}
	durationMin := sess.Data["duration"].(int)
	intensity := domain.IntensityMedium
	if v, ok := sess.Data["intensity"]; ok {
		intensity = domain.Intensity(v.(string))
	}

	user, _, err := h.userUC.GetProfile(ctx, domain.ProviderTelegram, strconv.FormatInt(fromID, 10))
	if err != nil || user == nil {
		h.log.Error("saveActivityEntry: failed to get user", zap.Error(err))
		h.reply(chatID, h.loc.T("error_internal"))
		return
	}

	if err := h.activityUC.SaveEntry(ctx, user.ID, activityType, customType, durationMin, intensity, recordedAt, chatID); err != nil {
		h.log.Error("saveActivityEntry: failed to save entry", zap.Error(err))
		h.reply(chatID, h.loc.T("error_internal"))
		return
	}

	impact := h.activityUC.EvaluateImpact(activityType, durationMin, intensity)

	typeLabel := h.activityTypeLabel(activityType, customType)
	timeStr := recordedAt.Format("02.01.2006, 15:04")

	var impactText string
	switch impact.RiskLevel {
	case domain.RiskLow:
		impactText = h.loc.T("activity_impact_low")
	case domain.RiskModerate:
		impactText = h.loc.T("activity_impact_moderate")
	case domain.RiskHigh:
		impactText = h.loc.T("activity_impact_high")
	}

	text := h.loc.T("activity_saved") + "\n\n" +
		typeLabel + " · " + strconv.Itoa(durationMin) + " мин\n" +
		"📅 " + timeStr + "\n\n" +
		impactText

	h.replyWithKeyboard(chatID, text, h.backToMenuKeyboard())
}

// activityTypeLabel возвращает локализованное название типа активности.
func (h *Handler) activityTypeLabel(at domain.ActivityType, customType string) string {
	if at == domain.ActivityOther && customType != "" {
		return "🏃 " + customType
	}
	key := "activity_type_" + string(at)
	return h.loc.T(key)
}

// handleActivityHistory показывает последние 5 записей активности.
func (h *Handler) handleActivityHistory(ctx context.Context, cb *tgbotapi.CallbackQuery) {
	user, _, err := h.userUC.GetProfile(ctx, domain.ProviderTelegram, strconv.FormatInt(cb.From.ID, 10))
	if err != nil || user == nil {
		h.log.Error("handleActivityHistory: failed to get user", zap.Error(err))
		h.reply(cb.Message.Chat.ID, h.loc.T("error_internal"))
		return
	}

	entries, err := h.activityUC.GetLastEntries(ctx, user.ID, 5)
	if err != nil {
		h.log.Error("handleActivityHistory: failed to get entries", zap.Error(err))
		h.reply(cb.Message.Chat.ID, h.loc.T("error_internal"))
		return
	}

	if len(entries) == 0 {
		h.replyWithKeyboard(cb.Message.Chat.ID, h.loc.T("last_empty_short"), h.backToMenuKeyboard())
		return
	}

	var sb strings.Builder
	sb.WriteString(h.loc.T("activity_history_title"))
	sb.WriteString("\n")
	for _, e := range entries {
		typeLabel := h.activityTypeLabel(e.ActivityType, e.CustomType)
		impact := h.activityUC.EvaluateImpact(e.ActivityType, e.DurationMin, e.Intensity)

		var impactIcon string
		switch impact.RiskLevel {
		case domain.RiskHigh:
			impactIcon = " ⚠️"
		case domain.RiskModerate:
			impactIcon = " ⚡"
		default:
			impactIcon = ""
		}

		sb.WriteString("  " + e.RecordedAt.Format("02.01 15:04") + " — " +
			typeLabel + " · " + strconv.Itoa(e.DurationMin) + " мин" + impactIcon + "\n")
	}

	h.replyWithKeyboard(cb.Message.Chat.ID, sb.String(), h.backToMenuKeyboard())
}
