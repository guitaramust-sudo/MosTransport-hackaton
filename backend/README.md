# VSM-400 Conductor Trainer — Backend

Бэкенд игры-тренажёра для обучения проводников высокоскоростной магистрали.
Одиночный режим: игрок проходит смену из процедурно генерируемых ситуаций, ведёт
текстовые диалоги с пассажирами (через GigaChat или Mock) и получает опыт за смену.

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
Все встроенные сценарии помечены `draft`: это демонстрационные гипотезы, а не
утверждённые рабочие процедуры. `POINTS_NAMESPACE=demo` позволяет их проходить,
но административное подтверждение результата запрещено. Для `approved`
сценария нужны `reviewer_id` и `source_refs`; до экспертной проверки таких
сценариев в поставке нет. В ином namespace старт смены вернёт `409`.

Публичная регистрация всегда создаёт обычного пользователя. Администратора
создаёт оператор отдельной локальной командой. Перед запуском задайте в своей
оболочке `ADMIN_BOOTSTRAP_EMAIL` и `ADMIN_BOOTSTRAP_PASSWORD` (не добавляйте
пароль в `.env` или Git), затем выполните из корня репозитория:

```bash
docker compose run --rm -e ADMIN_BOOTSTRAP_EMAIL -e ADMIN_BOOTSTRAP_PASSWORD \
  --entrypoint bootstrap-admin backend
```

Для локального Go запуска: `go run ./cmd/bootstrap-admin` из `backend/` с теми
же переменными и `DATABASE_URL`. Команда создаёт аккаунт либо повышает уже
существующий **только при совпадении его пароля**. После смены роли нужно
войти заново; старый access-токен перестаёт действовать. `ADMIN_EMAILS` больше
не используется.

Проверка:

```bash
curl http://localhost:8088/healthz
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
| `APP_ENV` | `development` | В `production` требуется уникальный `JWT_SECRET` и запрещён `GIGACHAT_INSECURE` |
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
| `CORS_ORIGINS` | localhost:8081 и localhost:19006 (HTTP) | Разрешённые источники Expo web, через запятую |

`GET /readyz` проверяет соединение с PostgreSQL. На `/auth/*` действует лимит
10 запросов в минуту на адрес подключения. Запросы логируются через `slog` с
`request_id`, HTTP-статусом и временем ответа. Вызовы GigaChat ограничены 30 с
и повторяются при 429, 5xx и сетевых ошибках. `GET /metrics` возвращает число
ошибок LLM как `llm_errors`. Пул PostgreSQL ограничен 10 соединениями, а SQL
запросы — 30 секундами.

## API

Полное описание — в [`docs/api.md`](docs/api.md), результаты локальной приёмки —
в [`docs/acceptance.md`](docs/acceptance.md). Кратко:

| Метод | Путь | Описание |
|---|---|---|
| POST | `/auth/register` | Регистрация |
| POST | `/auth/login` | Логин, возвращает JWT |
| POST | `/auth/refresh` | Обновление пары токенов |
| GET | `/api/profile` | Профиль текущего игрока |
| POST | `/api/session/start` | Начать смену |
| POST | `/api/session/simulations` | Начать демонстрационную смену с ветвлением |
| GET | `/api/session/simulations/{id}` | Получить состояние новой смены |
| POST | `/api/session/simulations/{id}/actions` | Выполнить выбор с `command_id` и версией состояния |
| GET | `/api/session/{id}` | Состояние смены |
| POST | `/api/session/{id}/finish` | Завершить смену (разбор + опыт) |
| GET | `/api/situation/{id}` | Детали ситуации (диалог, шкалы, таймер) |
| POST | `/api/situation/{id}/message` | Отправить текстовый ответ |
| POST | `/api/situation/{id}/escalate` | Вызвать адресата (`{"to":"medic"}`) |
| POST | `/api/situation/{id}/finish` | Закрыть ситуацию и получить скоринг |
| GET | `/api/leaderboard` | Топ игроков по XP |

Все `/api/*` требуют заголовок `Authorization: Bearer <token>`.
Новый simulation API пока содержит один конфигурируемый сервисный сценарий с
двумя ветками. Старый диалоговый API остаётся доступным до интеграции клиента.
Пространство, параллельные события и причинный разбор добавляются следующими
этапами; эта ветка не выдаётся за готовый симулятор смены.

## Архитектура

```
handler (HTTP) → service (игровая логика) → repo (PostgreSQL)
                       │
                       └── llm.LLMClient (GigaChat | Mock)
```

### Слои

- **handler** (`internal/handler`) — HTTP, парсинг запросов, коды ответов.
- **service** (`internal/service`) — генерация смены, диалоги, скоринг, начисление опыта.
- **content** (`internal/content`) — JSON-справочники ситуаций и пассажиров с валидацией.
- **repo** (`internal/repo`) — интерфейс `Store`; реализация в
  `internal/repo/postgres`.
- **llm** (`internal/llm`) — интерфейс `LLMClient` с двумя реализациями.

### LLM-абстракция

```go
type LLMClient interface {
    Chat(ctx, messages []llm.Message) (string, error)
    Classify(ctx, text string, categories []string) (string, error)
    ScoreDialogue(ctx, input llm.ScoringInput) (llm.ScoreResult, error)
}
```

- `Chat` получает **полную историю диалога** (system-промпт + последние N
  сообщений). История берётся из таблицы `messages`, поэтому бэкенд переживает
  рестарты.
- `ScoreDialogue` вызывается один раз при закрытии ситуации и возвращает
  наблюдаемые факты. Исход, XP и финальные шкалы вычисляет код.
- Переключение реализации — переменная `LLM_MODE`.

### Игровые механики

Ситуация выбирается из JSON-справочника вместе со случайным пассажиром.
Лояльность и безопасность стартуют с 50/50 и обновляются только после закрытия.
Ситуация закрывается по явному запросу, по таймеру или при завершении смены.
После одного вызова скоринга код вычисляет `success`, `partial`, `fail` или
`timeout` и сохраняет замечания, XP и финальные шкалы. Сейчас в справочниках
50 ситуаций и 50 пассажиров загружаются из JSON при старте приложения.

## Структура проекта

```
backend/
├── cmd/server/            # точка входа, роутер
├── internal/
│   ├── config/            # переменные окружения
│   ├── content/           # JSON-справочники и валидация
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
