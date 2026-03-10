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
	"github.com/gmtantsevov/shuggabuddy/internal/usecase/glucose"
	"github.com/gmtantsevov/shuggabuddy/internal/usecase/user"
	"github.com/gmtantsevov/shuggabuddy/pkg/config"
	"github.com/gmtantsevov/shuggabuddy/pkg/logger"
)

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

	userUC := user.New(userRepo, extAccRepo)
	glucoseUC := glucose.New(glucoseRepo)

	handler := telegram.NewHandler(bot, userUC, glucoseUC, loc, log)

	log.Info("starting bot...")
	handler.Run(ctx)

	log.Info("bot stopped gracefully")
}
