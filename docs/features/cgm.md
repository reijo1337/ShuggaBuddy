# CGM

## Описание

Интеграция с CGM-системами для автоматической синхронизации показаний глюкозы. Поддерживает два провайдера:

- **Nightscout** — open-source платформа для CGM-данных (FreeStyle Libre, Dexcom и др.)
- **LibreLinkUp** — облачный API Abbott для FreeStyle Libre через LibreView

Одновременно активен только один провайдер. Переключение — через отключение текущего и подключение нового.

## Подключение

- Раздел "📡 CGM" в профиле
- Экран выбора провайдера: Nightscout или LibreLinkUp
- Бот проверяет подключение перед сохранением

### Nightscout

- Пользователь вводит URL Nightscout и API Secret
- При первом открытии — пошаговая инструкция по настройке

### LibreLinkUp

- Пользователь вводит email и пароль от аккаунта LibreView
- Регион API определяется автоматически (redirect при логине)
- Предупреждение о хранении зашифрованных учётных данных

## Синхронизация

- Периодический поллинг каждые 5 мин (scheduler)
- Показания сохраняются в `glucose_readings` с `source='nightscout'` или `source='librelinkup'`
- Дедупликация через unique partial index в БД (отдельный индекс на каждый провайдер)
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

- `internal/domain/cgm.go` — сущности CGMConnection, CGMClient interface, интерфейс репозитория
- `pkg/nightscout/client.go` — HTTP-клиент Nightscout API (реализует CGMClient)
- `pkg/librelinkup/client.go` — HTTP-клиент LibreLinkUp API (реализует CGMClient)
- `pkg/crypto/token.go` — шифрование токенов
- `internal/repository/postgres/cgm_connection_repo.go` — реализация репозитория
- `internal/usecase/cgm/cgm_usecase.go` — бизнес-логика (мульти-провайдер через CGMClient)
- `internal/scheduler/cgm_sync.go` — фоновая синхронизация
- `internal/delivery/telegram/cgm.go` — Telegram-хэндлеры (выбор провайдера, подключение)
- `migrations/015_cgm_integration.sql` — миграция БД (Nightscout)
- `migrations/016_librelinkup.sql` — миграция БД (LibreLinkUp: region, dedup index)
