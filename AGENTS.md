# AGENTS.md

> **Правило:** При добавлении любой новой функциональности — обязательно обновить раздел [Bot Functionality](#bot-functionality) ниже. Без этого задача считается незавершённой.

## Project Overview

ShuggaBuddy — Telegram-бот, карманный помощник для людей с диабетом 1-го типа.

**Стек:** Go 1.25, PostgreSQL 14+, Telegram Bot API v5 (`go-telegram-bot-api`), pgx/v5, Zap logger

**Архитектура:** Clean Architecture

**Язык кода:** английский. Комментарии — только где логика неочевидна.

**Локализация:** русский (единственный язык, архитектура готова к мультиязычности)

## Commands

```bash
make run          # go run ./cmd/bot/main.go (требует .env)
make build        # собрать бинарник в bin/shuggabuddy
make test         # go test -v -race ./...
make lint         # golangci-lint в Docker
make fmt          # авто-форматирование через golangci-lint
make migrate      # применить все миграции (goose)
make migrate-down # откатить последнюю миграцию
```

Запустить один тест: `go test -v -run TestName ./internal/usecase/glucose/`

Сгенерировать моки: `go generate ./...`

Переменные окружения: см. `.env.example`.

---

## Bot Functionality

Ниже — список реализованного функционала. **Обновлять при каждом изменении** (и файл в `docs/features/`, и эту таблицу).

Детальные описания — в `docs/features/`. Читай нужный файл при работе с конкретной фичей.

| Фича | Файл | Краткое описание |
|------|------|-----------------|
| `/start` | [start.md](docs/features/start.md) | Регистрация, приветствие, главное меню |
| Профиль | [profile.md](docs/features/profile.md) | Настройки: единицы, диапазон, базальный инсулин, таймзона |
| Глюкоза | [glucose.md](docs/features/glucose.md) | Запись показаний, валидация, индикатор диапазона, история |
| Еда | [food.md](docs/features/food.md) | Запись углеводов (г / ХЕ), примечание, выбор времени |
| Инсулин | [insulin.md](docs/features/insulin.md) | Запись инъекций (болюс/базальный), валидация дозы |
| Калькулятор болюса | [bolus-calculator.md](docs/features/bolus-calculator.md) | Расчёт дозы по ICR/ISF/IOB из истории данных |
| Заметки | [notes.md](docs/features/notes.md) | Самочувствие, болезнь, стресс, свободный текст |
| Дневник | [diary.md](docs/features/diary.md) | Хронологический фид записей, навигация по дням |
| Рекомендации по дозам | [dose-advisor.md](docs/features/dose-advisor.md) | Тренд-анализ базального/болюсного, автоуведомления |

---

## Architecture Rules

Проект следует Clean Architecture с чётким направлением зависимостей:

```text
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
- Маршрутизация callback-запросов в `user.go` (`handleCallback`), интерфейсы зависимостей — в `deps.go`
- Вызывает usecase-слой через интерфейсы (`UserUseCase`, `GlucoseUseCase`, `BotAPI`), никогда — репозитории напрямую
- Моки delivery-слоя хранятся в `internal/delivery/telegram/mocks/`

### Pkg (`pkg/`)

- Утилиты без бизнес-логики (`config`, `logger`)
- Может использоваться любым слоем

### Scheduler (`internal/scheduler/`)

- Фоновые задачи: напоминания (reminders)
- Зависит от domain-интерфейсов и delivery (отправка сообщений)

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

## CI/CD

- GitHub Actions: `.github/workflows/ci.yml` (линтер), `.github/workflows/test.yml` (тесты)
- Docker-образа для деплоя нет — бот запускается как бинарник

---

## Gotchas

- `CLAUDE.md` — симлинк на `AGENTS.md`. Редактировать нужно `AGENTS.md`
- `golangci-lint` запускается через Docker (не требует локальной установки)
- `handleCallback` (роутер callback-кнопок) находится в `user.go`, а не в `handler.go`
- `analytics.go` и `session.go` в delivery — вспомогательные модули, не привязаны к отдельным хэндлерам

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
