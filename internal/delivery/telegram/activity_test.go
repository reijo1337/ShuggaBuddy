package telegram_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/gmtantsevov/shuggabuddy/internal/delivery/telegram"
	tgmocks "github.com/gmtantsevov/shuggabuddy/internal/delivery/telegram/mocks"
	"github.com/gmtantsevov/shuggabuddy/internal/domain"
)

func TestActivityHappyPath_Running(t *testing.T) {
	ctrl := gomock.NewController(t)
	bot := &spyBot{}
	userUC := tgmocks.NewMockUserUseCase(ctrl)
	activityUC := tgmocks.NewMockActivityUseCase(ctrl)
	loc := newTestLocalizer(t)

	h := newTestHandler(bot, userUC, nil, nil, nil, activityUC, loc)

	// Step 1: start activity flow
	cb := testCallback(123, 100, "menu:activity")
	h.HandleCallback(context.Background(), cb)
	require.GreaterOrEqual(t, len(bot.sent), 1)
	msg := bot.lastMessage()
	assert.Contains(t, msg.Text, "тип активности")

	// Step 2: select running
	bot.sent = nil
	cb = testCallback(123, 100, "activity:type:running")
	h.HandleCallback(context.Background(), cb)
	msg = bot.lastMessage()
	assert.Contains(t, msg.Text, "минут")

	// Step 3: select 30 min → now goes to intensity
	bot.sent = nil
	cb = testCallback(123, 100, "activity:dur:30")
	h.HandleCallback(context.Background(), cb)
	msg = bot.lastMessage()
	assert.Contains(t, msg.Text, "интенсивность")

	// Step 3.5: select medium intensity
	bot.sent = nil
	cb = testCallback(123, 100, "activity:intensity:medium")
	h.HandleCallback(context.Background(), cb)
	msg = bot.lastMessage()
	assert.Contains(t, msg.Text, "Когда")

	// Step 4: select "now"
	userUC.EXPECT().GetProfile(gomock.Any(), domain.ProviderTelegram, "123").
		Return(testUser, testAcc, nil)
	activityUC.EXPECT().SaveEntry(gomock.Any(), int64(1), domain.ActivityRunning, "", 30, domain.IntensityMedium, gomock.Any(), int64(100)).
		Return(nil)
	activityUC.EXPECT().EvaluateImpact(domain.ActivityRunning, 30, domain.IntensityMedium).
		Return(domain.GlycemicImpact{RiskLevel: domain.RiskHigh, MonitorHours: 4})

	bot.sent = nil
	cb = testCallback(123, 100, "activity:time:now")
	h.HandleCallback(context.Background(), cb)
	msg = bot.lastMessage()
	assert.Contains(t, msg.Text, "Активность записана")
	assert.Contains(t, msg.Text, "Высокий риск")
}

func TestActivityHappyPath_Other_CustomType(t *testing.T) {
	ctrl := gomock.NewController(t)
	bot := &spyBot{}
	userUC := tgmocks.NewMockUserUseCase(ctrl)
	activityUC := tgmocks.NewMockActivityUseCase(ctrl)
	loc := newTestLocalizer(t)

	h := newTestHandler(bot, userUC, nil, nil, nil, activityUC, loc)

	// Step 1: start + select "other" type
	h.HandleCallback(context.Background(), testCallback(123, 456, "menu:activity"))
	bot.sent = nil
	h.HandleCallback(context.Background(), testCallback(123, 456, "activity:type:other"))
	msg := bot.lastMessage()
	assert.Contains(t, msg.Text, "название активности")

	// Step 2: set up session and enter custom type
	sess := telegram.NewSession(telegram.SessionActivity, telegram.StepActivityCustom)
	sess.Data["type"] = "other"
	h.SetSession(456, sess)

	bot.sent = nil
	h.HandleSessionInput(context.Background(), testMessage(123, 456, "Фехтование"), sess)
	msg = bot.lastMessage()
	assert.Contains(t, msg.Text, "минут")
}

func TestActivityDurationQuickButtons(t *testing.T) {
	durations := []struct {
		callback string
		minutes  int
	}{
		{"activity:dur:15", 15},
		{"activity:dur:30", 30},
		{"activity:dur:45", 45},
		{"activity:dur:60", 60},
		{"activity:dur:90", 90},
	}

	for _, tt := range durations {
		t.Run(tt.callback, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			bot := &spyBot{}
			activityUC := tgmocks.NewMockActivityUseCase(ctrl)
			loc := newTestLocalizer(t)

			h := newTestHandler(bot, nil, nil, nil, nil, activityUC, loc)
			sess := telegram.NewSession(telegram.SessionActivity, telegram.StepActivityDuration)
			sess.Data["type"] = "running"
			h.SetSession(100, sess)

			h.HandleCallback(context.Background(), testCallback(123, 100, tt.callback))
			msg := bot.lastMessage()
			assert.Contains(t, msg.Text, "интенсивность")
		})
	}
}

func TestActivityDurationManualInput_Valid(t *testing.T) {
	ctrl := gomock.NewController(t)
	bot := &spyBot{}
	activityUC := tgmocks.NewMockActivityUseCase(ctrl)
	loc := newTestLocalizer(t)

	h := newTestHandler(bot, nil, nil, nil, nil, activityUC, loc)
	sess := telegram.NewSession(telegram.SessionActivity, telegram.StepActivityDuration)
	sess.Data["type"] = "running"
	h.SetSession(100, sess)

	h.HandleSessionInput(context.Background(), testMessage(123, 100, "25"), sess)
	msg := bot.lastMessage()
	assert.Contains(t, msg.Text, "интенсивность")
}

func TestActivityDurationManualInput_Invalid(t *testing.T) {
	ctrl := gomock.NewController(t)
	bot := &spyBot{}
	activityUC := tgmocks.NewMockActivityUseCase(ctrl)
	loc := newTestLocalizer(t)

	h := newTestHandler(bot, nil, nil, nil, nil, activityUC, loc)
	sess := telegram.NewSession(telegram.SessionActivity, telegram.StepActivityDuration)
	sess.Data["type"] = "running"
	h.SetSession(100, sess)

	h.HandleSessionInput(context.Background(), testMessage(123, 100, "abc"), sess)
	msg := bot.lastMessage()
	assert.Contains(t, msg.Text, "от 1 до 600")
}

func TestActivityDurationManualInput_OutOfRange(t *testing.T) {
	ctrl := gomock.NewController(t)
	bot := &spyBot{}
	activityUC := tgmocks.NewMockActivityUseCase(ctrl)
	loc := newTestLocalizer(t)

	h := newTestHandler(bot, nil, nil, nil, nil, activityUC, loc)
	sess := telegram.NewSession(telegram.SessionActivity, telegram.StepActivityDuration)
	sess.Data["type"] = "running"
	h.SetSession(100, sess)

	h.HandleSessionInput(context.Background(), testMessage(123, 100, "700"), sess)
	msg := bot.lastMessage()
	assert.Contains(t, msg.Text, "от 1 до 600")
}

func TestActivityImpactLow(t *testing.T) {
	ctrl := gomock.NewController(t)
	bot := &spyBot{}
	userUC := tgmocks.NewMockUserUseCase(ctrl)
	activityUC := tgmocks.NewMockActivityUseCase(ctrl)
	loc := newTestLocalizer(t)

	h := newTestHandler(bot, userUC, nil, nil, nil, activityUC, loc)
	sess := telegram.NewSession(telegram.SessionActivity, telegram.StepActivityTime)
	sess.Data["type"] = "yoga"
	sess.Data["duration"] = 15
	sess.Data["intensity"] = "low"
	h.SetSession(100, sess)

	userUC.EXPECT().GetProfile(gomock.Any(), domain.ProviderTelegram, "123").
		Return(testUser, testAcc, nil)
	activityUC.EXPECT().SaveEntry(gomock.Any(), int64(1), domain.ActivityYoga, "", 15, domain.IntensityLow, gomock.Any(), int64(100)).
		Return(nil)
	activityUC.EXPECT().EvaluateImpact(domain.ActivityYoga, 15, domain.IntensityLow).
		Return(domain.GlycemicImpact{RiskLevel: domain.RiskLow, MonitorHours: 1})

	h.HandleCallback(context.Background(), testCallback(123, 100, "activity:time:now"))
	msg := bot.lastMessage()
	assert.Contains(t, msg.Text, "Незначительное влияние")
}

func TestActivityHistory(t *testing.T) {
	ctrl := gomock.NewController(t)
	bot := &spyBot{}
	userUC := tgmocks.NewMockUserUseCase(ctrl)
	activityUC := tgmocks.NewMockActivityUseCase(ctrl)
	loc := newTestLocalizer(t)

	h := newTestHandler(bot, userUC, nil, nil, nil, activityUC, loc)

	userUC.EXPECT().GetProfile(gomock.Any(), domain.ProviderTelegram, "123").
		Return(testUser, testAcc, nil)

	entries := []domain.ActivityEntry{
		{
			ID: 1, UserID: 1, ActivityType: domain.ActivityRunning,
			DurationMin: 30, Intensity: domain.IntensityMedium, RecordedAt: time.Now(),
		},
	}
	activityUC.EXPECT().GetLastEntries(gomock.Any(), int64(1), 5).Return(entries, nil)
	activityUC.EXPECT().EvaluateImpact(domain.ActivityRunning, 30, domain.IntensityMedium).
		Return(domain.GlycemicImpact{RiskLevel: domain.RiskHigh, MonitorHours: 4})

	h.HandleCallback(context.Background(), testCallback(123, 100, "activity:history"))
	msg := bot.lastMessage()
	assert.Contains(t, msg.Text, "История активности")
	assert.Contains(t, msg.Text, "Бег")
}

func TestActivityInMixedFeed(t *testing.T) {
	now := time.Now()
	entries := []domain.ActivityEntry{
		{
			ID: 1, ActivityType: domain.ActivityRunning,
			DurationMin: 30, Intensity: domain.IntensityMedium, RecordedAt: now,
		},
	}
	activityRows := []string{
		"  " + now.Format("02.01 15:04") + " — 🏃 Бег · 30 мин",
	}

	rows := telegram.BuildMixedHistory(nil, nil, nil, entries, activityRows, domain.UnitsMmol, "ммоль/л", 5)
	require.Len(t, rows, 1)
	assert.Contains(t, rows[0], "Бег")
	assert.Contains(t, rows[0], "30 мин")
}
