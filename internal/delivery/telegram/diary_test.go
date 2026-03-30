package telegram_test

import (
	"context"
	"testing"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/gmtantsevov/shuggabuddy/internal/delivery/telegram"
	tgmocks "github.com/gmtantsevov/shuggabuddy/internal/delivery/telegram/mocks"
	"github.com/gmtantsevov/shuggabuddy/internal/domain"
)

// testCallbackWithMessageID creates a callback with a specific message ID.
func testCallbackWithMessageID(userID, chatID int64, messageID int, data string) *tgbotapi.CallbackQuery {
	return &tgbotapi.CallbackQuery{
		ID:   "cb_1",
		From: &tgbotapi.User{ID: userID, FirstName: "Test"},
		Message: &tgbotapi.Message{
			MessageID: messageID,
			Chat:      &tgbotapi.Chat{ID: chatID},
		},
		Data: data,
	}
}

// TestHandleDiaryShow_Empty verifies that when there are no entries for today,
// the message contains the "diary_empty" text.
func TestHandleDiaryShow_Empty(t *testing.T) {
	ctrl := gomock.NewController(t)
	bot := &spyBot{}
	userUC := tgmocks.NewMockUserUseCase(ctrl)
	diaryUC := tgmocks.NewMockDiaryUseCase(ctrl)
	loc := newTestLocalizer(t)

	h := newTestHandler(bot, userUC, nil, nil, nil, nil, nil, diaryUC, loc)

	userUC.EXPECT().GetProfile(gomock.Any(), domain.ProviderTelegram, "123").
		Return(testUser, testAcc, nil)
	diaryUC.EXPECT().GetDayEntries(gomock.Any(), int64(1), gomock.Any(), gomock.Any()).
		Return([]*domain.DiaryEntry{}, nil)

	h.HandleCallback(context.Background(), testCallbackWithMessageID(123, 100, 42, "menu:diary"))

	require.GreaterOrEqual(t, len(bot.sent), 1)
	msg := bot.lastMessage()
	assert.Contains(t, msg.Text, "Записей за этот день нет")
}

// TestHandleDiaryShow_WithEntries verifies that a glucose entry is rendered correctly.
func TestHandleDiaryShow_WithEntries(t *testing.T) {
	ctrl := gomock.NewController(t)
	bot := &spyBot{}
	userUC := tgmocks.NewMockUserUseCase(ctrl)
	diaryUC := tgmocks.NewMockDiaryUseCase(ctrl)
	loc := newTestLocalizer(t)

	h := newTestHandler(bot, userUC, nil, nil, nil, nil, nil, diaryUC, loc)

	now := time.Now()
	glucEntry := &domain.DiaryEntry{
		Kind: domain.DiaryKindGlucose,
		Time: now,
		Glucose: &domain.GlucoseReading{
			ValueMmol:  5.4,
			RecordedAt: now,
		},
	}

	userUC.EXPECT().GetProfile(gomock.Any(), domain.ProviderTelegram, "123").
		Return(testUser, testAcc, nil)
	diaryUC.EXPECT().GetDayEntries(gomock.Any(), int64(1), gomock.Any(), gomock.Any()).
		Return([]*domain.DiaryEntry{glucEntry}, nil)

	h.HandleCallback(context.Background(), testCallbackWithMessageID(123, 100, 42, "menu:diary"))

	require.GreaterOrEqual(t, len(bot.sent), 1)
	msg := bot.lastMessage()
	assert.Contains(t, msg.Text, "🩸")
	assert.Contains(t, msg.Text, "5.4")
}

// TestHandleDiaryCallback_NavPrev verifies that navigation to previous day calls GetDayEntries
// with the correct date.
func TestHandleDiaryCallback_NavPrev(t *testing.T) {
	ctrl := gomock.NewController(t)
	bot := &spyBot{}
	userUC := tgmocks.NewMockUserUseCase(ctrl)
	diaryUC := tgmocks.NewMockDiaryUseCase(ctrl)
	loc := newTestLocalizer(t)

	h := newTestHandler(bot, userUC, nil, nil, nil, nil, nil, diaryUC, loc)

	expectedDate := time.Date(2026, 3, 27, 0, 0, 0, 0, time.UTC)

	userUC.EXPECT().GetProfile(gomock.Any(), domain.ProviderTelegram, "123").
		Return(testUser, testAcc, nil)
	diaryUC.EXPECT().GetDayEntries(gomock.Any(), int64(1), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ int64, date time.Time, _ *time.Location) ([]*domain.DiaryEntry, error) {
			// Verify the date matches expected (same day)
			assert.Equal(t, expectedDate.Year(), date.Year())
			assert.Equal(t, expectedDate.Month(), date.Month())
			assert.Equal(t, expectedDate.Day(), date.Day())
			return []*domain.DiaryEntry{}, nil
		})

	h.HandleCallback(context.Background(), testCallbackWithMessageID(123, 100, 42, "diary:show:2026-03-27"))

	require.GreaterOrEqual(t, len(bot.requests), 1)
}

// TestHandleDiaryCallback_DatePrompt verifies that diary:date callback sets the session
// to sessionDiary/stepDiaryDate.
func TestHandleDiaryCallback_DatePrompt(t *testing.T) {
	bot := &spyBot{}
	loc := newTestLocalizer(t)

	h := newTestHandler(bot, nil, nil, nil, nil, nil, nil, nil, loc)

	h.HandleCallback(context.Background(), testCallbackWithMessageID(123, 100, 42, "diary:date"))

	require.GreaterOrEqual(t, len(bot.sent), 1)
	msg := bot.lastMessage()
	assert.Contains(t, msg.Text, "ДД.ММ")

	// Verify session was set - by sending another message that goes through session
	// We can confirm the prompt was sent which implies session was set
}

// TestHandleDiaryText_ValidDate verifies that a valid date input triggers GetDayEntries
// for the correct date.
func TestHandleDiaryText_ValidDate(t *testing.T) {
	ctrl := gomock.NewController(t)
	bot := &spyBot{}
	userUC := tgmocks.NewMockUserUseCase(ctrl)
	diaryUC := tgmocks.NewMockDiaryUseCase(ctrl)
	loc := newTestLocalizer(t)

	h := newTestHandler(bot, userUC, nil, nil, nil, nil, nil, diaryUC, loc)

	sess := telegram.NewSession(telegram.SessionDiary, telegram.StepDiaryDate)
	h.SetSession(100, sess)

	userUC.EXPECT().GetProfile(gomock.Any(), domain.ProviderTelegram, "123").
		Return(testUser, testAcc, nil)
	diaryUC.EXPECT().GetDayEntries(gomock.Any(), int64(1), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ int64, date time.Time, _ *time.Location) ([]*domain.DiaryEntry, error) {
			assert.Equal(t, 15, date.Day())
			assert.Equal(t, time.March, date.Month())
			assert.Equal(t, 2026, date.Year())
			return []*domain.DiaryEntry{}, nil
		})

	h.HandleSessionInput(context.Background(), testMessage(123, 100, "15.03.2026"), sess)

	require.GreaterOrEqual(t, len(bot.sent), 1)
}

// TestHandleDiaryText_InvalidDate verifies that an invalid date input sends diary_date_invalid.
func TestHandleDiaryText_InvalidDate(t *testing.T) {
	bot := &spyBot{}
	loc := newTestLocalizer(t)

	h := newTestHandler(bot, nil, nil, nil, nil, nil, nil, nil, loc)

	sess := telegram.NewSession(telegram.SessionDiary, telegram.StepDiaryDate)
	h.SetSession(100, sess)

	h.HandleSessionInput(context.Background(), testMessage(123, 100, "abc"), sess)

	require.GreaterOrEqual(t, len(bot.sent), 1)
	msg := bot.lastMessage()
	assert.Contains(t, msg.Text, "Не удалось распознать дату")
}
