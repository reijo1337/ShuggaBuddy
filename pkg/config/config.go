// Package config отвечает за загрузку конфигурации приложения из переменных окружения.
package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

// Config содержит параметры конфигурации приложения.
type Config struct {
	TelegramToken    string
	DatabaseURL      string
	LogLevel         string
	DefaultLang      string
	CGMEncryptionKey string
}

// Load загружает конфигурацию из .env файла и переменных окружения.
// Переменные окружения имеют приоритет над значениями из .env.
func Load() (*Config, error) {
	// Игнорируем ошибку — .env может отсутствовать (например, в Docker)
	_ = godotenv.Load()

	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("config.Load: TELEGRAM_BOT_TOKEN is required")
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return nil, fmt.Errorf("config.Load: DATABASE_URL is required")
	}

	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		logLevel = "info"
	}

	defaultLang := os.Getenv("DEFAULT_LANG")
	if defaultLang == "" {
		defaultLang = "ru"
	}

	return &Config{
		TelegramToken:    token,
		DatabaseURL:      dbURL,
		LogLevel:         logLevel,
		DefaultLang:      defaultLang,
		CGMEncryptionKey: os.Getenv("CGM_ENCRYPTION_KEY"),
	}, nil
}
