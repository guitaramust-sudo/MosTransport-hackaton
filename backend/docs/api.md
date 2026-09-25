# API тренажёра

Базовый адрес при `docker compose up`: `http://localhost:8088`. Все пути
`/api/*` требуют `Authorization: Bearer <access_token>`.

## Авторизация

| Метод | Путь | Тело |
|---|---|---|
| POST | `/auth/register` | `{"email":"...","username":"...","password":"..."}` |
| POST | `/auth/login` | `{"email":"...","password":"..."}` |
| POST | `/auth/refresh` | `{"refresh_token":"..."}` |

Ответ регистрации и входа содержит `player` и `tokens.access_token`,
`tokens.refresh_token`, `tokens.expires_in`.

## Смена и ситуации

| Метод | Путь | Тело | Ответ |
|---|---|---|---|
| POST | `/api/session/start` | `{}` | `session`, `situations` |
| GET | `/api/session/{id}` | — | `session`, `situations` |
| POST | `/api/session/{id}/finish` | `{}` | `total_xp`, `situations` с разбором |
| GET | `/api/situation/{id}` | — | `situation`, `messages` |
| POST | `/api/situation/{id}/message` | `{"text":"..."}` | ответ пассажира, таймер и неизменные до закрытия шкалы |
| POST | `/api/situation/{id}/escalate` | `{"to":"medic"}` | список `escalations` |
| POST | `/api/situation/{id}/finish` | `{}` | `outcome`, `tone`, `conveyed`, `missed`, `xp`, `remarks`, финальные шкалы |

Адресаты эскалации: `train_chief`, `ptb`, `police`, `medic`, `ambulance`.
Повторное закрытие ситуации возвращает `409`. Повторное завершение смены
возвращает сохранённый разбор без повторного начисления XP.

`GET /api/profile` возвращает профиль и компетенции; `GET /api/leaderboard`
возвращает таблицу лидеров. `GET /healthz` — проверка процесса,
`GET /readyz` — готовность БД, `GET /metrics` — счётчик ошибок LLM.
