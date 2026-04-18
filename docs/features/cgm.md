# CGM (Nightscout)

## Описание

Интеграция с Nightscout — open-source платформой для CGM-данных. Поддерживает FreeStyle Libre, Dexcom и другие CGM-системы.

## Подключение

- Раздел "📡 CGM (Nightscout)" в профиле
- Пользователь вводит URL Nightscout и API Secret
- Бот проверяет подключение перед сохранением
- При первом открытии — пошаговая инструкция по настройке

## Синхронизация

- Периодический поллинг каждые 5 мин (scheduler)
- Показания сохраняются в `glucose_readings` с `source='nightscout'`
- Дедупликация через unique partial index в БД
- Первая синхронизация забирает данные за последние 24 часа

## Тренд-стрелки

CGM-показания отображаются с тренд-стрелками в дневнике и истории глюкозы:

| Тренд | Emoji |
|-------|-------|
| DoubleUp | ⬆⬆ |
| SingleUp | ⬆ |
| FortyFiveUp | ↗ |
| Flat | → |
| FortyFiveDown | ↘ |
| SingleDown | ⬇ |
| DoubleDown | ⬇⬇ |

## Безопасность

- API-токены шифруются AES-256-GCM
- Ключ шифрования в переменной окружения `CGM_ENCRYPTION_KEY`
- Без ключа CGM-функции gracefully отключены

## Файлы

- `internal/domain/cgm.go` — сущность CGMConnection, интерфейс репозитория
- `pkg/nightscout/client.go` — HTTP-клиент Nightscout API
- `pkg/crypto/token.go` — шифрование токенов
- `internal/repository/postgres/cgm_connection_repo.go` — реализация репозитория
- `internal/usecase/cgm/cgm_usecase.go` — бизнес-логика
- `internal/scheduler/cgm_sync.go` — фоновая синхронизация
- `internal/delivery/telegram/cgm.go` — Telegram-хэндлеры
- `migrations/015_cgm_integration.sql` — миграция БД
