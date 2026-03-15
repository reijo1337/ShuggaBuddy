# ShuggaBuddy

[![CI](https://github.com/reijo1337/ShuggaBuddy/actions/workflows/ci.yml/badge.svg)](https://github.com/reijo1337/ShuggaBuddy/actions/workflows/ci.yml)
[![Tests](https://github.com/reijo1337/ShuggaBuddy/actions/workflows/test.yml/badge.svg)](https://github.com/reijo1337/ShuggaBuddy/actions/workflows/test.yml)

Telegram-бот — карманный помощник для людей с сахарным диабетом 1 типа.

Ключевая идея: бот не заменяет врача, но существенно снижает когнитивную нагрузку на пациента — освобождает от ручного ведения записей,
помогает «на месте» оценить еду и подобрать дозу, выявляет паттерны в данных и своевременно предупреждает о рисках.

## Ключевые принципы

* Простота использования — минимум шагов для записи любого события
* Персонализация — бот учитывается индивидуальные настройки: ИЧ, КИ, целевой диапазон
* Прозрачность рекомендаций — бот объясняет логику каждого расчёта
* Безопасность — все рекомендации сопровождаются оговоркой о консультации с врачом
* LLM-центричность — ключевые смысловые задачи выполняет языковая модель

## Стек

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
