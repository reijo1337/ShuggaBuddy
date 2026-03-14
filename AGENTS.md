# AGENTS.md

## Project Overview

ShuggaBuddy — Telegram-бот, карманный помощник для людей с диабетом 1-го типа.

**Стек:** Go 1.25, PostgreSQL 14+, Telegram Bot API v5 (`go-telegram-bot-api`), pgx/v5, Zap logger

**Архитектура:** Clean Architecture

**Язык кода:** английский. Комментарии — только где логика неочевидна.

**Локализация:** русский (единственный язык, архитектура готова к мультиязычности)

---

## Architecture Rules

Проект следует Clean Architecture с чётким направлением зависимостей:

```
delivery → usecase → domain ← repository
```

Обратные зависимости запрещены.

### Domain (`internal/domain/`)

- Чистые сущности и интерфейсы репозиториев
- Ноль зависимостей от внешних пакетов
- Интерфейсы репозиториев определяются здесь, реализации — в `internal/repository/`

### Usecase (`internal/usecase/`)

- Бизнес-логика, валидация, конвертация единиц
- Зависит только от domain-интерфейсов через dependency injection
- Один пакет на агрегат (`user/`, `glucose/`)

### Repository (`internal/repository/postgres/`)

- Реализации domain-интерфейсов через pgx/v5
- Пул соединений через `pgxpool`
- Только CRUD-операции, никакой бизнес-логики

### Delivery (`internal/delivery/telegram/`)

- Взаимодействие через inline-кнопки, единственная команда — `/start`
- Маршрутизация callback-запросов в `handler.go`, интерфейсы зависимостей — в `deps.go`
- Вызывает usecase-слой через интерфейсы (`UserUseCase`, `GlucoseUseCase`, `BotAPI`), никогда — репозитории напрямую
- Моки delivery-слоя хранятся в `internal/delivery/telegram/mocks/`

### Pkg (`pkg/`)

- Утилиты без бизнес-логики (`config`, `logger`)
- Может использоваться любым слоем

### I18n (`internal/i18n/`)

- Реализация локализации через `Localizer`
- Используется в delivery-слое для формирования ответов пользователю

---

## Testing

- Фреймворк: `testify` (ассерты), `go.uber.org/mock` (моки)
- Моки генерируются через `//go:generate mockgen`, хранятся в `internal/domain/mocks/` и `internal/delivery/telegram/mocks/`
- Тесты — в `*_test.go` рядом с тестируемым кодом
- Юнит-тесты обязательны для usecase- и delivery-слоёв
- Табличные тесты (table-driven) для валидации и граничных случаев
- Не мокать то, что можно проверить напрямую
- Запуск: `make test`

---

## Database Migrations

- Инструмент: goose
- Директория: `migrations/`
- Именование: `NNN_описание.sql` (трёхзначный номер, snake_case)
- Каждая миграция содержит `-- +goose Up` и `-- +goose Down`
- Down-миграция обязательна и должна корректно откатывать изменения
- Новые таблицы/индексы — новая миграция, не правка существующей
- Применение: `make migrate`, откат: `make migrate-down`

---

## Localization (i18n)

- Файлы переводов: `locales/{lang}.yaml`
- Ключи — snake_case на английском
- Интерполяция через `fmt.Sprintf`
- Все пользовательские сообщения — через `i18n.Localizer`, не хардкодить строки
- Сейчас один язык (ru), архитектура поддерживает мультиязычность

---

## Workflows

**Важно**: При любых задачах не нужно ничего коммитить.

### New Bot Action (inline-кнопка)

1. Добавить ключи переводов в `locales/ru.yaml`
2. Добавить handler в `internal/delivery/telegram/`
3. Зарегистрировать callback в роутере (`handleCallback` в `user.go`)
4. Если нужен новый метод usecase — добавить в интерфейс в `deps.go` и перегенерировать моки: `go generate ./internal/delivery/telegram/...`
5. Если нужна бизнес-логика — добавить метод в usecase
6. Если нужны данные — добавить метод в domain-интерфейс и реализацию в repository
7. Написать тесты на usecase и delivery

### New Entity

1. Определить struct и интерфейс репозитория в `internal/domain/`
2. Создать миграцию в `migrations/`
3. Реализовать репозиторий в `internal/repository/postgres/`
4. Создать usecase в `internal/usecase/`
5. Сгенерировать моки: `go generate ./...`
6. Написать тесты
