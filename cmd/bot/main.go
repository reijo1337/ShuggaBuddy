// Package main — точка входа Telegram-бота ShuggaBuddy.
package main

import (
	"context"
	"os/signal"
	"syscall"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"github.com/gmtantsevov/shuggabuddy/internal/delivery/telegram"
	"github.com/gmtantsevov/shuggabuddy/internal/i18n"
	"github.com/gmtantsevov/shuggabuddy/internal/repository/postgres"
	"github.com/gmtantsevov/shuggabuddy/internal/scheduler"
	"github.com/gmtantsevov/shuggabuddy/internal/usecase/activity"
	"github.com/gmtantsevov/shuggabuddy/internal/usecase/diary"
	"github.com/gmtantsevov/shuggabuddy/internal/usecase/food"
	"github.com/gmtantsevov/shuggabuddy/internal/usecase/glucose"
	"github.com/gmtantsevov/shuggabuddy/internal/usecase/insulin"
	"github.com/gmtantsevov/shuggabuddy/internal/usecase/note"
	"github.com/gmtantsevov/shuggabuddy/internal/usecase/user"
	"github.com/gmtantsevov/shuggabuddy/pkg/config"
	"github.com/gmtantsevov/shuggabuddy/pkg/logger"
)

// telegramMessenger adapts tgbotapi.BotAPI to scheduler.Messenger.
type telegramMessenger struct {
	bot *tgbotapi.BotAPI
	log *zap.Logger
}

func (m *telegramMessenger) SendReminder(chatID int64, text string) error {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "Markdown"
	_, err := m.bot.Send(msg)
	if err != nil {
		m.log.Error("failed to send reminder", zap.Error(err), zap.Int64("chat_id", chatID))
	}
	return err
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic("failed to load config: " + err.Error())
	}

	log, err := logger.New(cfg.LogLevel)
	if err != nil {
		panic("failed to init logger: " + err.Error())
	}
	defer func() { _ = log.Sync() }()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Error("failed to connect to database", zap.Error(err))
		return
	}
	defer pool.Close()

	if pingErr := pool.Ping(ctx); pingErr != nil {
		log.Error("failed to ping database", zap.Error(pingErr))
		return
	}
	log.Info("connected to database")

	loc, err := i18n.NewLocalizer("locales", cfg.DefaultLang)
	if err != nil {
		log.Error("failed to init localizer", zap.Error(err))
		return
	}

	bot, err := tgbotapi.NewBotAPI(cfg.TelegramToken)
	if err != nil {
		log.Error("failed to init telegram bot", zap.Error(err))
		return
	}
	log.Info("authorized on telegram", zap.String("bot", bot.Self.UserName))

	userRepo := postgres.NewUserRepo(pool)
	extAccRepo := postgres.NewExternalAccountRepo(pool)
	glucoseRepo := postgres.NewGlucoseRepo(pool)
	foodRepo := postgres.NewFoodRepo(pool)
	insulinRepo := postgres.NewInsulinRepo(pool)
	activityRepo := postgres.NewActivityRepo(pool)
	reminderRepo := postgres.NewReminderRepo(pool)
	noteRepo := postgres.NewNoteRepository(pool)

	userUC := user.New(userRepo, extAccRepo)
	glucoseUC := glucose.New(glucoseRepo)
	foodUC := food.New(foodRepo)
	insulinUC := insulin.New(insulinRepo)
	activityUC := activity.New(activityRepo, glucoseRepo, reminderRepo)
	noteUC := note.New(noteRepo)
	diaryUC := diary.New(glucoseRepo, foodRepo, insulinRepo, activityRepo, noteRepo)

	handler := telegram.NewHandler(bot, userUC, glucoseUC, foodUC, insulinUC, activityUC, noteUC, diaryUC, loc, log)

	messenger := &telegramMessenger{bot: bot, log: log}
	reminderScheduler := scheduler.NewReminderScheduler(reminderRepo, activityRepo, glucoseRepo, messenger, log)
	go reminderScheduler.Run(ctx)

	log.Info("starting bot...")
	handler.Run(ctx)

	log.Info("bot stopped gracefully")
}
