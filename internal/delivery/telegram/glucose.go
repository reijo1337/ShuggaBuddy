package telegram

import (
	"context"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.uber.org/zap"

	"github.com/gmtantsevov/shuggabuddy/internal/domain"
	fooduc "github.com/gmtantsevov/shuggabuddy/internal/usecase/food"
	glucoseuc "github.com/gmtantsevov/shuggabuddy/internal/usecase/glucose"
)

// handleGlucoseStart переводит пользователя в режим ожидания ввода сахара.
func (h *Handler) handleGlucoseStart(cb *tgbotapi.CallbackQuery) {
	sess := newSession(sessionGlucose, stepGlucoseValue)
	h.sessions.Store(cb.Message.Chat.ID, sess)
	h.replyWithKeyboard(cb.Message.Chat.ID, h.loc.T("glucose_prompt"), h.backToMenuKeyboard())
}

// handleGlucoseStep обрабатывает текстовый ввод уровня сахара.
func (h *Handler) handleGlucoseStep(ctx context.Context, msg *tgbotapi.Message, _ *Session) {
	h.sessions.Delete(msg.Chat.ID)
	text := strings.TrimSpace(msg.Text)

	value, err := strconv.ParseFloat(text, 64)
	if err != nil {
		h.replyWithKeyboard(msg.Chat.ID, h.loc.T("glucose_invalid_short"), h.backToMenuKeyboard())
		return
	}

	user, _, err := h.userUC.GetProfile(ctx, domain.ProviderTelegram, strconv.FormatInt(msg.From.ID, 10))
	if err != nil || user == nil {
		h.log.Error("handleGlucoseStep: failed to get user", zap.Error(err))
		h.reply(msg.Chat.ID, h.loc.T("error_internal"))
		return
	}

	if err := h.glucUC.SaveReading(ctx, user.ID, value, user.Units); err != nil {
		h.log.Warn("handleGlucoseStep: invalid reading",
			zap.Error(err),
			zap.Float64("value", value),
		)
		var rangeMsg string
		if user.Units == domain.UnitsMgdl {
			rangeMsg = h.loc.T("glucose_out_of_range_mgdl")
		} else {
			rangeMsg = h.loc.T("glucose_out_of_range_mmol")
		}
		h.replyWithKeyboard(msg.Chat.ID, rangeMsg, h.backToMenuKeyboard())
		return
	}

	unitsLabel := h.unitsLabel(string(user.Units))

	var displayValue string
	var valueMmol float64
	if user.Units == domain.UnitsMgdl {
		displayValue = strconv.FormatFloat(value, 'f', 0, 64)
		valueMmol = value / domain.MmolToMgdl
	} else {
		displayValue = strconv.FormatFloat(value, 'f', 1, 64)
		valueMmol = value
	}

	indicator := glucoseStatusEmoji(valueMmol, user.TargetMinMmol, user.TargetMaxMmol)

	h.replyWithKeyboard(
		msg.Chat.ID,
		h.loc.T("glucose_saved", displayValue, unitsLabel)+" "+indicator,
		h.backToMenuKeyboard(),
	)
}

// buildMixedHistory merges glucose readings, food entries, and insulin doses,
// sorts by time descending, and returns at most `limit` formatted rows.
// On equal timestamps, glucose sorts before food before insulin.
func buildMixedHistory(
	glucReadings []domain.GlucoseReading,
	foodEntries []domain.FoodEntry,
	insulinDoses []domain.InsulinDose,
	activityEntries []domain.ActivityEntry,
	activityRows []string,
	units domain.Units,
	unitsLabel string,
	limit int,
) []string {
	type entry struct {
		t    int64 // unix timestamp for sorting
		kind int   // 0 = glucose, 1 = food, 2 = insulin
		text string
	}

	var all []entry

	for _, r := range glucReadings {
		all = append(all, entry{
			t:    r.RecordedAt.Unix(),
			kind: 0,
			text: formatGlucoseRow(r, units, unitsLabel),
		})
	}

	for _, e := range foodEntries {
		all = append(all, entry{
			t:    e.EatenAt.Unix(),
			kind: 1,
			text: formatFoodRow(e),
		})
	}

	for _, d := range insulinDoses {
		all = append(all, entry{
			t:    d.RecordedAt.Unix(),
			kind: 2,
			text: formatInsulinRow(d),
		})
	}

	for i, e := range activityEntries {
		text := ""
		if i < len(activityRows) {
			text = activityRows[i]
		}
		all = append(all, entry{
			t:    e.RecordedAt.Unix(),
			kind: 3,
			text: text,
		})
	}

	// Sort: newest first; on tie, glucose before food.
	for i := 1; i < len(all); i++ {
		for j := i; j > 0; j-- {
			a, b := all[j-1], all[j]
			if a.t < b.t || (a.t == b.t && a.kind > b.kind) {
				all[j-1], all[j] = all[j], all[j-1]
			}
		}
	}

	if len(all) > limit {
		all = all[:limit]
	}

	rows := make([]string, len(all))
	for i, e := range all {
		rows[i] = e.text
	}
	return rows
}

func glucoseStatusEmoji(valueMmol, minMmol, maxMmol float64) string {
	switch domain.GlucoseStatus(valueMmol, minMmol, maxMmol) {
	case "low":
		return "🔴"
	case "high":
		return "🟡"
	default:
		return "🟢"
	}
}

func formatGlucoseRow(r domain.GlucoseReading, units domain.Units, unitsLabel string) string {
	return "  " + r.RecordedAt.Format("02.01 15:04") + " — 🩸 " +
		glucoseuc.FormatValue(r.ValueMmol, units) + " " + unitsLabel + trendArrow(r.Trend)
}

func trendArrow(trend *string) string {
	if trend == nil {
		return ""
	}
	switch *trend {
	case "DoubleUp":
		return " ⬆⬆"
	case "SingleUp":
		return " ⬆"
	case "FortyFiveUp":
		return " ↗"
	case "Flat":
		return " →"
	case "FortyFiveDown":
		return " ↘"
	case "SingleDown":
		return " ⬇"
	case "DoubleDown":
		return " ⬇⬇"
	default:
		return ""
	}
}

func formatFoodRow(e domain.FoodEntry) string {
	note := ""
	if e.Note != "" {
		note = " (" + e.Note + ")"
	}
	return "  " + e.EatenAt.Format("02.01 15:04") + " — 🍽 " +
		fooduc.FormatCarbs(e.CarbsGrams) + "г" + note
}

func formatInsulinRow(d domain.InsulinDose) string {
	drug := ""
	if d.Drug != "" {
		drug = " (" + d.Drug + ")"
	}
	return "  " + d.RecordedAt.Format("02.01 15:04") + " — 💉 " +
		formatDoseUnits(d.DoseUnits) + " ед. " + string(d.InsulinType) + drug
}
