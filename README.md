# ShuggaBuddy

[![CI](https://github.com/reijo1337/ShuggaBuddy/actions/workflows/ci.yml/badge.svg)](https://github.com/reijo1337/ShuggaBuddy/actions/workflows/ci.yml)
[![Tests](https://github.com/reijo1337/ShuggaBuddy/actions/workflows/test.yml/badge.svg)](https://github.com/reijo1337/ShuggaBuddy/actions/workflows/test.yml)

Telegram-бот — карманный помощник для людей с сахарным диабетом 1 типа.

## Функциональность

Взаимодействие с ботом происходит через inline-кнопки. Единственная команда — `/start`.

После `/start` бот показывает главное меню с кнопками:

- **Профиль** — имя, единицы измерения, дата регистрации
- **Единицы: ммоль/л** — переключение между ммоль/л и мг/дл (текущие единицы отображаются в кнопке)
- **Новая запись** — бот ожидает ввод уровня сахара текстом
- **Последние 5 записей** — список недавних измерений

Каждый экран содержит кнопку «В меню» для возврата.

## Требования

- Go 1.25+
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
  delivery/       — Telegram-обработчики (inline-кнопки, callback-роутинг)
  i18n/           — локализация
pkg/
  config/         — конфигурация
  logger/         — логирование
locales/          — файлы локализации (YAML)
migrations/       — SQL-миграции (goose)
```
