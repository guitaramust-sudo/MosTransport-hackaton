# VSM-400 Conductor Trainer — Backend

Бэкенд игры-тренажёра для обучения проводников высокоскоростной магистрали.
В одном процессе живут три независимых игровых потока (последовательные
диалоговые смены, ветвящаяся demo-симуляция, real-time вагон по WebSocket) и
поверх вагона — учебный курс из глав/уроков с теорией, тестами, призовыми
баллами и push-уведомлениями.

## Архитектура

- [docs/component-diagram.pdf](docs/component-diagram.pdf) — из чего состоит
  бэкенд и как связаны сервисы, контент и БД.
- [docs/sequence-diagram.pdf](docs/sequence-diagram.pdf) — сквозной пример:
  игрок проходит учебный урок от карты курса до начисления награды.

## Исследование и опрос

Игровые решения (реалистичные ситуации важнее длины сессии, таймер только
там, где реально важна скорость, достижения весомее чистого рейтинга,
сессия ≤15 минут) основаны на [опросе 22 бывших/действующих
проводников](docs/research-discovery.md) о реальной работе и обучении.
Короткая выжимка:

- **Кто отвечал:** 20/22 — студотряды, 16/22 со стажем до года, только 2
  когда-либо работали на скоростном/высокоскоростном поезде. Выборка
  смещена в сторону новичков — это про восприятие трудностей и пожеланий к
  тренировке, **не статистика ВСМ**.
- **Что встречалось чаще всего:** жалобы на сервис/оборудование (21/22),
  техническая неисправность (20/22), несколько проблем одновременно (19/22).
  **Что называли самым трудным** — не то же самое: чаще всего трудной
  считали медицинскую ситуацию (6/22), хотя технические неисправности
  встречались чаще всего.
- **Чего не хватает в подготовке:** материалы не соответствуют реальной
  работе (11/22), редкие ситуации почти невозможно потренировать (10/22),
  нет возможности сразу попробовать снова после ошибки (3/22).
- **Желаемый формат:** реалистичные ситуации (20/22) и разбор последствий
  (13/22) важнее длины; 17/22 считают приемлемой сессию до 15 минут; таймер
  большинство (15/22) хочет только там, где скорость реально важна в
  работе, не на каждом шаге.
- **Мотивация вернуться:** достижения/значки отметили 15/22, рейтинг — 9/22
  — рейтинг уместен, но не главный повод для большинства.

Полный анализ с оговорками о размере/смещении выборки — в
[docs/research-discovery.md](docs/research-discovery.md); необработанные
ответы — в [docs/survey-raw-data.json](docs/survey-raw-data.json). Опрос
**не подтверждает** конкретные процедуры ВСМ, веса Safety/Loyalty или
оптимальную длительность сессии — только направление, проверка нужна на
прототипе с целевыми сотрудниками.

## Стек

- **Go 1.22+**, роутер `go-chi/chi/v5`
- **PostgreSQL 16**, драйвер `pgx/v5`
- **JWT** (`golang-jwt/jwt/v5`), пароли — `bcrypt`
- **LLM** — GigaChat (реальный) или встроенный Mock (офлайн-демо)
- **Push** — интерфейс `push.Sender`; сейчас логирующий мок, реальный
  Expo/FCM подключается отдельным файлом, когда будут credentials

## Быстрый старт

```bash
cp .env.example .env
# впишите в .env свой ADMIN_BOOTSTRAP_EMAIL / ADMIN_BOOTSTRAP_PASSWORD
docker compose up --build
```

Поднимает PostgreSQL 16 и backend. Бэкенд сам выполняет миграции при старте
и, если в `.env` заданы `ADMIN_BOOTSTRAP_EMAIL`/`ADMIN_BOOTSTRAP_PASSWORD`,
сам создаёт (или повышает уже существующий, **только при совпадении
пароля**) аккаунт с ролью `admin` — при **каждом** старте, без отдельной
команды. Не заданы переменные — шаг пропускается, ничего не ломается. По
умолчанию включён mock-режим LLM — игра работает без ключей и сети.

Проверка, что сервис жив:

```bash
curl http://localhost:8088/healthz   # ok
curl http://localhost:8088/readyz    # ok (проверяет соединение с БД)
```

## Первоначальная настройка

1. **Переменные окружения** — скопируйте `.env.example` в `.env` и задайте
   хотя бы `ADMIN_BOOTSTRAP_EMAIL`/`ADMIN_BOOTSTRAP_PASSWORD` (для локального
   запуска без Docker остальные уже имеют рабочие значения по умолчанию).
2. **Поднять стек** — `docker compose up --build`.
3. **Получить токен админа**:
   ```bash
   curl -X POST localhost:8088/auth/login -H 'Content-Type: application/json' \
     -d '{"email":"you@example.com","password":"your-strong-password"}'
   ```
   Публичной регистрации нет — это единственный способ войти без
   `POST /admin/players`.
4. **Завести обычных игроков** (только под токеном admin):
   ```bash
   curl -X POST localhost:8088/admin/players -H "Authorization: Bearer $TOKEN" \
     -H 'Content-Type: application/json' \
     -d '{"email":"passenger1@example.com","username":"passenger1","password":"минимум6символов","brigade_name":"Бригада-1"}'
   ```
   `brigade_name` — просто текст (нет справочника бригад), по нему же
   фильтруется `GET /api/leaderboards?scope=brigade&group_id=...`.
5. Готово — новый игрок логинится через `/auth/login` своими данными.

Если нужен админ без перезапуска сервера или без Docker — та же логика
доступна как отдельная команда: `go run ./cmd/bootstrap-admin` из `backend/`
с переменными, экспортированными в оболочку (`.env` этот путь не читает).

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
| `GIGACHAT_CLIENT_ID` / `_SECRET` | — | Креды GigaChat (только для `gigachat`) |
| `GIGACHAT_AUTH_URL` / `_API_URL` / `_MODEL` | см. `.env.example` | Настройки GigaChat |
| `GIGACHAT_INSECURE` | `false` | Пропуск проверки TLS (только для локальной разработки, запрещено в `production`) |
| `SITUATIONS_PER_SESSION` | `4` | Число ситуаций в диалоговой смене |
| `CORS_ORIGINS` | localhost:8081, localhost:19006 | Разрешённые источники, через запятую |
| `POINTS_NAMESPACE` | `demo` | `demo` или `official` — изолированные реестры очков рейтинга |
| `ADMIN_BOOTSTRAP_EMAIL` / `ADMIN_BOOTSTRAP_PASSWORD` | — | Если заданы — сервер сам создаёт/повышает админа при каждом старте |

`GET /readyz` проверяет соединение с PostgreSQL. На `/auth/*` действует лимит
10 запросов в минуту на адрес подключения. Запросы логируются через `slog` с
`request_id`, HTTP-статусом и временем ответа. Вызовы GigaChat ограничены 30 с
и повторяются при 429, 5xx и сетевых ошибках. `GET /metrics` возвращает число
ошибок LLM как `llm_errors`. Пул PostgreSQL ограничен 10 соединениями, а SQL
запросы — 30 секундами.

## Игровые потоки

Три независимых потока делят таблицы `sessions`/`situations`, но не код друг
друга (подробности и связи — на component-диаграмме):

1. **Диалоговая смена** (`/api/session/*`, `/api/situation/*`) — ситуация из
   каталога + случайный пассажир, свободный диалог через LLM, скоринг при
   закрытии.
2. **Ветвящаяся demo-симуляция** (`/api/session/simulations/*`) — один
   захардкоженный сценарий с двумя ветками и скрытым сигналом, отдельный
   граф событий.
3. **Real-time вагон** (`/api/session/wagon/*`, WebSocket) — символьные
   якоря, пассажиры сидят и ждут, ситуации спавнятся по вероятности из весов
   выбранного уровня (`/api/wagon/levels`), сам разговор с пассажиром идёт
   через тот же движок, что и поток 1.
4. **Учебный курс** (`/api/learning/*`, поверх потока 3) — главы и уроки с
   теорией/тестами/практикой в вагоне (класс `first`), идемпотентным
   начислением XP, бейджа и призовых баллов, push о разблокировке
   следующего урока.

## API

Все `/api/*` и `/admin/*` требуют заголовок `Authorization: Bearer <token>`,
кроме WebSocket handshake (токен в `token` query). `/admin/*` — только роль
`admin`.

| Метод | Путь | Описание |
|---|---|---|
| POST | `/auth/login` | Логин, возвращает JWT (регистрации нет) |
| POST | `/auth/refresh` | Обновление пары токенов |
| GET | `/api/profile` | Профиль текущего игрока |
| GET | `/api/notifications` | Демонстрационные уведомления |
| GET | `/api/challenges/weekly` | Прогресс недельного demo-челленджа |
| GET | `/api/leaderboard` | Топ игроков (с percentile) |
| GET | `/api/leaderboards` | Рейтинг по компании/депо/бригаде (`?scope=&group_id=`) |
| POST | `/api/session/start` | Начать диалоговую смену |
| GET / POST | `/api/session/{id}`, `/api/session/{id}/finish` | Состояние / завершение смены |
| GET / POST | `/api/situation/{id}`, `/message`, `/escalate`, `/finish` | Диалог, эскалация, скоринг ситуации |
| POST | `/api/session/simulations` | Начать ветвящуюся demo-смену |
| GET / POST | `/api/session/simulations/{id}`, `/actions`, `/dialogue`, `/result` | Состояние/действия/разбор demo-смены |
| GET | `/api/wagon/classes` | Классы вагона (`standard`, `first`) и их доступность |
| GET | `/api/wagon/levels` | Уровни вагона: locked/unlocked/passed для игрока |
| POST | `/api/session/wagon/start` | Начать смену в вагоне по `level_id` |
| WS | `/api/wagon/{id}/ws?token=...` | Снимки состояния, физические команды |
| GET | `/api/learning/map` | Карта курса: главы, уроки, статус |
| GET | `/api/learning/lessons/{id}` | Теория + вопросы урока (без ключа ответов) |
| POST | `/api/learning/lessons/{id}/answers` | Ответ на вопрос теории/практики |
| POST | `/api/learning/lessons/{id}/practice` | Начать практику (сессия в вагоне `first`) |
| POST | `/api/learning/lessons/{id}/finalize` | Завершить урок, идемпотентно начислить награду |
| GET | `/api/me/learning` | Свой прогресс по урокам + призовой баланс |
| POST | `/api/me/push-subscriptions` | Зарегистрировать device token для push |
| POST | `/admin/players` | Создать аккаунт игрока (email/username/password/`brigade_name`) |
| POST | `/admin/users` | Legacy: upsert внешнего (SSO/HR) пользователя без пароля |
| GET | `/admin/users/{id}/learning-summary` | Сводка по игроку: компетенции + прогресс по урокам/уровням |
| POST | `/admin/sessions/{id}/approve` | Утвердить завершённую диалоговую смену |
| POST | `/admin/users/{id}/prize-credits/demo-seed` | Тестовая призовая запись (проверка 6-дневного истечения без ожидания) |

## Структура проекта

```
backend/
├── cmd/
│   ├── server/            # точка входа, роутер, фоновые тикеры
│   └── bootstrap-admin/   # CLI для создания/повышения админа
├── internal/
│   ├── config/            # переменные окружения
│   ├── content/           # JSON-справочники (сценарии, вагон, уровни, курс) и валидация
│   ├── domain/            # модели
│   ├── repo/              # интерфейс Store
│   │   └── postgres/      # реализация + миграции (go:embed)
│   ├── service/           # бизнес-логика всех потоков
│   ├── llm/                # LLMClient: gigachat + mock
│   ├── push/               # push.Sender: лог-мок + место для Expo/FCM
│   ├── handler/            # HTTP/WS-обработчики
│   └── middleware/         # JWT, rate limit
├── docs/
│   ├── component-diagram.pdf
│   ├── sequence-diagram.pdf
│   ├── research-discovery.md   # анализ опроса 22 проводников
│   └── survey-raw-data.json    # необработанные ответы опроса
├── docker-compose.yml
└── Dockerfile
```
