# VSM-400 Conductor Trainer — Backend

Бэкенд игры-тренажёра для обучения проводников высокоскоростной магистрали.
Одиночный режим: игрок проходит смену из процедурно генерируемых ситуаций, ведёт
текстовые диалоги с пассажирами (через GigaChat) и получает опыт по компетенциям.

## Стек

- **Go 1.22+**, роутер `go-chi/chi/v5`
- **PostgreSQL 16**, драйвер `pgx/v5`
- **JWT** (`golang-jwt/jwt/v5`), пароли — `bcrypt`
- **LLM** — GigaChat (реальный) или встроенный Mock (офлайн-демо)

## Быстрый старт

```bash
docker compose up --build
```

Поднимает PostgreSQL 16 и backend. Бэкенд сам выполняет миграции при старте.
По умолчанию включён mock-режим LLM — игра работает без ключей и сети.

Проверка:

```bash
curl http://localhost:8080/healthz
# ok
```

## Локальный запуск без Docker

1. Поднимите PostgreSQL 16 и создайте базу `vsm`.
2. Задайте переменные окружения (см. `.env.example`).
3. Запустите:

```bash
go run ./cmd/server
```

## Переменные окружения

| Переменная | По умолчанию | Описание |
|---|---|---|
| `DATABASE_URL` | `postgres://vsm:vsm@localhost:5432/vsm?sslmode=disable` | DSN PostgreSQL |
| `SERVER_PORT` | `8080` | Порт HTTP |
| `JWT_SECRET` | `dev-secret-change-me` | Секрет подписи JWT |
| `JWT_ACCESS_TTL` | `24h` | Время жизни access-токена |
| `JWT_REFRESH_TTL` | `720h` | Время жизни refresh-токена |
| `LLM_MODE` | `mock` | `mock` или `gigachat` |
| `GIGACHAT_CLIENT_ID` | — | Client ID GigaChat |
| `GIGACHAT_CLIENT_SECRET` | — | Client Secret GigaChat |
| `GIGACHAT_AUTH_URL` | `https://ngw.devices.sberbank.ru:9443/api/v2/oauth` | OAuth endpoint |
| `GIGACHAT_API_URL` | `https://gigachat.devices.sberbank.ru/api/v1` | API base |
| `GIGACHAT_MODEL` | `GigaChat-2` | Модель |
| `GIGACHAT_INSECURE` | `false` | Пропуск проверки TLS (только для локальной разработки) |
| `SITUATIONS_PER_SESSION` | `4` | Число ситуаций в смене |
| `SITUATION_TIMEOUT` | `120s` | Таймер на ситуацию |
| `MAX_TURNS_PER_SITUATION` | `3` | Реплик игрока до закрытия ситуации |

## API

Полное описание — в [`docs/api.md`](docs/api.md). Кратко:

| Метод | Путь | Описание |
|---|---|---|
| POST | `/auth/register` | Регистрация |
| POST | `/auth/login` | Логин, возвращает JWT |
| POST | `/auth/refresh` | Обновление пары токенов |
| GET | `/api/profile` | Профиль текущего игрока |
| POST | `/api/session/start` | Начать смену |
| GET | `/api/session/{id}` | Состояние смены |
| POST | `/api/session/{id}/finish` | Завершить смену (разбор + опыт) |
| GET | `/api/situation/{id}` | Детали ситуации (диалог, шкалы, таймер) |
| POST | `/api/situation/{id}/message` | Отправить текстовый ответ |
| GET | `/api/leaderboard` | Топ игроков по XP |

Все `/api/*` требуют заголовок `Authorization: Bearer <token>`.

## Архитектура

```
handler (HTTP) → service (игровая логика) → repo (PostgreSQL)
                       │
                       └── llm.LLMClient (GigaChat | Mock)
```

### Слои

- **handler** (`internal/handler`) — HTTP, парсинг запросов, коды ответов.
- **service** (`internal/service`) — вся игровая логика: генерация смены,
  диалоги, шкалы, классификация, начисление опыта.
- **repo** (`internal/repo`) — интерфейс `Store`; реализация в
  `internal/repo/postgres`.
- **llm** (`internal/llm`) — интерфейс `LLMClient` с двумя реализациями.

### LLM-абстракция

```go
type LLMClient interface {
    Chat(ctx, messages []llm.Message) (string, error)
    Classify(ctx, text string, categories []string) (string, error)
}
```

- `Chat` получает **полную историю диалога** (system-промпт + последние N
  сообщений). История берётся из таблицы `messages`, поэтому бэкенд переживает
  рестарты.
- `Classify` — отдельный вызов с **минимальным контекстом** (только реплика
  игрока + список категорий) для экономии токенов.
- Переключение реализации — переменная `LLM_MODE`.

### Игровые механики

У каждой ситуации две шкалы — **лояльность пассажира** и **рейтинг
безопасности** (старт 50/50). Каждая реплика игрока классифицируется в одну из
категорий и по-разному двигает шкалы:

| Категория | Лояльность | Безопасность | Компетенция |
|---|---|---|---|
| эмпатия | +8 | +1 | communication |
| давление | −10 | +2 | safety |
| отстранение | −8 | −4 | — |
| вызов помощи | −2 | +10 | safety |
| неверное действие | −8 | −12 | — |

Ситуация закрывается после `MAX_TURNS_PER_SITUATION` реплик игрока или по
истечении таймера. Смена заканчивается, когда закрыты все ситуации; при
завершении считается разбор и начисляется общий опыт и опыт по компетенциям.

## Структура проекта

```
backend/
├── cmd/server/            # точка входа, роутер
├── internal/
│   ├── config/            # переменные окружения
│   ├── domain/            # модели
│   ├── repo/              # интерфейс Store
│   │   └── postgres/      # реализация + миграции (go:embed)
│   ├── service/           # бизнес-логика
│   ├── llm/               # LLMClient: gigachat + mock
│   ├── handler/           # HTTP-обработчики
│   └── middleware/        # JWT
├── docker-compose.yml
├── Dockerfile
└── docs/api.md
```
