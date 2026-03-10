# ShuggaBuddy

Telegram-бот — карманный помощник для людей с сахарным диабетом 1 типа.

## Функциональность (итерация 0)

- `/start` — приветствие, создание профиля
- `/help` — список команд
- `/profile` — просмотр профиля
- `/setunits` — выбор единиц измерения (ммоль/л / мг/дл)
- `/glucose <значение>` — запись уровня сахара
- `/last` — последние 5 записей

## Требования

- Go 1.22+
- PostgreSQL 14+
- [goose](https://github.com/pressly/goose) — для миграций

## Запуск

### 1. Создание базы данных

```bash
createdb shuggabuddy
```

### 2. Настройка окружения

```bash
cp .env.example .env
# Заполни TELEGRAM_BOT_TOKEN и DATABASE_URL
```

### 3. Применение миграций

```bash
# Установка goose (если ещё не установлен)
go install github.com/pressly/goose/v3/cmd/goose@latest

# Применение миграций
make migrate
```

### 4. Запуск бота

```bash
make run
```

## Тесты

```bash
make test
```

## Архитектура

Проект следует принципам Clean Architecture:

```
cmd/bot/          — точка входа
internal/
  domain/         — сущности и интерфейсы
  usecase/        — бизнес-логика
  repository/     — реализации репозиториев (PostgreSQL)
  delivery/       — Telegram-обработчики
  i18n/           — локализация
pkg/
  config/         — конфигурация
  logger/         — логирование
locales/          — файлы локализации (YAML)
migrations/       — SQL-миграции (goose)
```
