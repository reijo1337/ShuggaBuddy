package telegram_test

import (
	"context"
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/gmtantsevov/shuggabuddy/internal/delivery/telegram"
	tgmocks "github.com/gmtantsevov/shuggabuddy/internal/delivery/telegram/mocks"
	"github.com/gmtantsevov/shuggabuddy/internal/domain"
)

func TestHandleNoteStart(t *testing.T) {
	ctrl := gomock.NewController(t)
	bot := &spyBot{}
	noteUC := tgmocks.NewMockNoteUseCase(ctrl)
	loc := newTestLocalizer(t)

	h := newTestHandler(bot, nil, nil, nil, nil, nil, noteUC, nil, nil, nil, loc)
	h.HandleCallback(context.Background(), testCallback(123, 100, "menu:note"))

	require.Len(t, bot.sent, 1)
	msg := bot.lastMessage()
	assert.Contains(t, msg.Text, "тип заметки")

	kb := msg.ReplyMarkup.(tgbotapi.InlineKeyboardMarkup)
	// 4 note type buttons (potentially spread across rows)
	var allButtons []tgbotapi.InlineKeyboardButton
	for _, row := range kb.InlineKeyboard {
		allButtons = append(allButtons, row...)
	}
	assert.Len(t, allButtons, 4)
	assert.Equal(t, "note:type:wellbeing", *allButtons[0].CallbackData)
	assert.Equal(t, "note:type:illness", *allButtons[1].CallbackData)
	assert.Equal(t, "note:type:stress", *allButtons[2].CallbackData)
	assert.Equal(t, "note:type:free", *allButtons[3].CallbackData)
}

func TestHandleNoteCallback_TypeWellbeing(t *testing.T) {
	ctrl := gomock.NewController(t)
	bot := &spyBot{}
	noteUC := tgmocks.NewMockNoteUseCase(ctrl)
	loc := newTestLocalizer(t)

	h := newTestHandler(bot, nil, nil, nil, nil, nil, noteUC, nil, nil, nil, loc)
	h.HandleCallback(context.Background(), testCallback(123, 100, "note:type:wellbeing"))

	require.Len(t, bot.sent, 1)
	msg := bot.lastMessage()
	assert.Contains(t, msg.Text, "себя чувствуешь")

	kb := msg.ReplyMarkup.(tgbotapi.InlineKeyboardMarkup)
	var allButtons []tgbotapi.InlineKeyboardButton
	for _, row := range kb.InlineKeyboard {
		allButtons = append(allButtons, row...)
	}
	assert.Len(t, allButtons, 4)
	assert.Equal(t, "note:wellbeing:good", *allButtons[0].CallbackData)
	assert.Equal(t, "note:wellbeing:normal", *allButtons[1].CallbackData)
	assert.Equal(t, "note:wellbeing:bad", *allButtons[2].CallbackData)
	assert.Equal(t, "note:wellbeing:sick", *allButtons[3].CallbackData)
}

func TestHandleNoteCallback_TypeFree(t *testing.T) {
	ctrl := gomock.NewController(t)
	bot := &spyBot{}
	noteUC := tgmocks.NewMockNoteUseCase(ctrl)
	loc := newTestLocalizer(t)

	h := newTestHandler(bot, nil, nil, nil, nil, nil, noteUC, nil, nil, nil, loc)
	h.HandleCallback(context.Background(), testCallback(123, 100, "note:type:free"))

	require.Len(t, bot.sent, 1)
	msg := bot.lastMessage()
	assert.Contains(t, msg.Text, "Добавь текст")

	// Keyboard should contain a skip button
	kb := msg.ReplyMarkup.(tgbotapi.InlineKeyboardMarkup)
	var allButtons []tgbotapi.InlineKeyboardButton
	for _, row := range kb.InlineKeyboard {
		allButtons = append(allButtons, row...)
	}
	var skipFound bool
	for _, btn := range allButtons {
		if btn.CallbackData != nil && *btn.CallbackData == "note:skip" {
			skipFound = true
		}
	}
	assert.True(t, skipFound, "expected note:skip button in keyboard")
}

func TestHandleNoteCallback_WellbeingGood(t *testing.T) {
	ctrl := gomock.NewController(t)
	bot := &spyBot{}
	noteUC := tgmocks.NewMockNoteUseCase(ctrl)
	loc := newTestLocalizer(t)

	h := newTestHandler(bot, nil, nil, nil, nil, nil, noteUC, nil, nil, nil, loc)

	// Pre-set session as if note:type:wellbeing was already selected
	sess := telegram.NewSession(telegram.SessionNote, telegram.StepNoteWellbeing)
	sess.Data["note_type"] = "wellbeing"
	h.SetSession(100, sess)

	h.HandleCallback(context.Background(), testCallback(123, 100, "note:wellbeing:good"))

	require.Len(t, bot.sent, 1)
	msg := bot.lastMessage()
	assert.Contains(t, msg.Text, "Добавь текст")

	// Keyboard should contain a skip button
	kb := msg.ReplyMarkup.(tgbotapi.InlineKeyboardMarkup)
	var allButtons []tgbotapi.InlineKeyboardButton
	for _, row := range kb.InlineKeyboard {
		allButtons = append(allButtons, row...)
	}
	var skipFound bool
	for _, btn := range allButtons {
		if btn.CallbackData != nil && *btn.CallbackData == "note:skip" {
			skipFound = true
		}
	}
	assert.True(t, skipFound, "expected note:skip button in keyboard")
}

func TestHandleNoteCallback_Skip(t *testing.T) {
	ctrl := gomock.NewController(t)
	bot := &spyBot{}
	userUC := tgmocks.NewMockUserUseCase(ctrl)
	noteUC := tgmocks.NewMockNoteUseCase(ctrl)
	loc := newTestLocalizer(t)

	h := newTestHandler(bot, userUC, nil, nil, nil, nil, noteUC, nil, nil, nil, loc)

	sess := telegram.NewSession(telegram.SessionNote, telegram.StepNoteText)
	sess.Data["note_type"] = "free"
	h.SetSession(100, sess)

	userUC.EXPECT().GetProfile(gomock.Any(), domain.ProviderTelegram, "123").
		Return(testUser, testAcc, nil)
	noteUC.EXPECT().SaveNote(gomock.Any(), int64(1), domain.NoteTypeFree, (*domain.WellbeingValue)(nil), (*string)(nil)).
		Return(nil)

	h.HandleCallback(context.Background(), testCallback(123, 100, "note:skip"))

	require.GreaterOrEqual(t, len(bot.sent), 1)
	msg := bot.lastMessage()
	assert.Contains(t, msg.Text, "Заметка сохранена")
}

func TestHandleNoteText_WithText(t *testing.T) {
	ctrl := gomock.NewController(t)
	bot := &spyBot{}
	userUC := tgmocks.NewMockUserUseCase(ctrl)
	noteUC := tgmocks.NewMockNoteUseCase(ctrl)
	loc := newTestLocalizer(t)

	h := newTestHandler(bot, userUC, nil, nil, nil, nil, noteUC, nil, nil, nil, loc)

	sess := telegram.NewSession(telegram.SessionNote, telegram.StepNoteText)
	sess.Data["note_type"] = "free"
	h.SetSession(100, sess)

	expectedText := "Чувствую себя нормально"
	userUC.EXPECT().GetProfile(gomock.Any(), domain.ProviderTelegram, "123").
		Return(testUser, testAcc, nil)
	noteUC.EXPECT().SaveNote(gomock.Any(), int64(1), domain.NoteTypeFree, (*domain.WellbeingValue)(nil), gomock.Any()).
		DoAndReturn(func(_ interface{}, _ int64, _ domain.NoteType, _ *domain.WellbeingValue, text *string) error {
			require.NotNil(t, text)
			assert.Equal(t, expectedText, *text)
			return nil
		})

	h.HandleSessionInput(context.Background(), testMessage(123, 100, expectedText), sess)

	require.GreaterOrEqual(t, len(bot.sent), 1)
	msg := bot.lastMessage()
	assert.Contains(t, msg.Text, "Заметка сохранена")
}

func TestHandleNoteText_WellbeingWithText(t *testing.T) {
	ctrl := gomock.NewController(t)
	bot := &spyBot{}
	userUC := tgmocks.NewMockUserUseCase(ctrl)
	noteUC := tgmocks.NewMockNoteUseCase(ctrl)
	loc := newTestLocalizer(t)

	h := newTestHandler(bot, userUC, nil, nil, nil, nil, noteUC, nil, nil, nil, loc)

	sess := telegram.NewSession(telegram.SessionNote, telegram.StepNoteText)
	sess.Data["note_type"] = "wellbeing"
	sess.Data["note_wellbeing"] = "good"
	h.SetSession(100, sess)

	expectedText := "Сегодня отличный день"
	expectedWellbeing := domain.WellbeingGood

	userUC.EXPECT().GetProfile(gomock.Any(), domain.ProviderTelegram, "123").
		Return(testUser, testAcc, nil)
	noteUC.EXPECT().SaveNote(
		gomock.Any(),
		int64(1),
		domain.NoteTypeWellbeing,
		&expectedWellbeing,
		gomock.Any(),
	).DoAndReturn(func(_ interface{}, _ int64, _ domain.NoteType, wellbeing *domain.WellbeingValue, text *string) error {
		require.NotNil(t, wellbeing)
		assert.Equal(t, domain.WellbeingGood, *wellbeing)
		require.NotNil(t, text)
		assert.Equal(t, expectedText, *text)
		return nil
	})

	h.HandleSessionInput(context.Background(), testMessage(123, 100, expectedText), sess)

	require.GreaterOrEqual(t, len(bot.sent), 1)
	msg := bot.lastMessage()
	assert.Contains(t, msg.Text, "Заметка сохранена")
}
