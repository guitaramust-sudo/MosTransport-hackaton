# Админка

Публичной регистрации нет — единственный способ завести обычный аккаунт
теперь через админа (`POST /admin/players`). Все `/admin/*` требуют
`Authorization: Bearer <access_token>` от игрока с ролью `admin`, иначе `403`.

## Как получить первого админа

Роль `admin` никому не выдаётся автоматически, но и отдельную команду
запускать не нужно — сервер сам бутстрапит админа при **каждом старте**,
если в `.env` заданы обе переменные:

```bash
# backend/.env — файл уже в .gitignore
ADMIN_BOOTSTRAP_EMAIL=you@example.com
ADMIN_BOOTSTRAP_PASSWORD=your-strong-password
```

Дальше просто `docker compose up` — при старте `cmd/server` создаёт этот
аккаунт с ролью `admin`, либо повышает уже существующий с таким email, но
**только если его пароль совпадает** с указанным (защита от захвата чужого
аккаунта чужим `.env`). Не заданы переменные — шаг просто пропускается,
ничего не ломается. После повышения роли уже существующего аккаунта нужно
залогиниться заново — старый access-токен с ролью `user` для `/admin/*` не
подойдёт.

Тот же бутстрап есть и как отдельная CLI-команда, если нужно поднять админа
без перезапуска сервера или без Docker:
```bash
docker compose run --rm --entrypoint bootstrap-admin backend
# либо локально без Docker, из backend/ (переменные экспортировать в оболочку,
# .env этот путь не читает):
# go run ./cmd/bootstrap-admin
```

Получить токен:
```bash
curl -X POST localhost:8088/auth/login -H 'Content-Type: application/json' \
  -d '{"email":"you@example.com","password":"your-strong-password"}'
```
`tokens.access_token` из ответа — это и есть Bearer-токен для всего ниже.

## Эндпоинты

| Метод | Путь | Назначение |
|---|---|---|
| `POST` | `/admin/players` | Создать обычный (логин/пароль) аккаунт игрока — только так теперь появляются новые пользователи |
| `POST` | `/admin/users` | Legacy: upsert внешнего (SSO/HR) пользователя без пароля — отдельный, не связанный с `/admin/players` механизм |
| `GET` | `/admin/users/{id}/learning-summary` | Полная сводка по игроку: компетенции + прогресс по уровням вагона |
| `POST` | `/admin/sessions/{id}/approve` | Пометить завершённую смену как `validation_status=approved` |

### `POST /admin/players` — создать игрока

```json
{
  "email": "passenger42@example.com",
  "username": "passenger42",
  "password": "минимум 6 символов",
  "brigade_name": "Бригада-3"
}
```

Все четыре поля обязательны — без `brigade_name` или с паролем короче 6
символов вернётся `400`. `brigade_name` — это просто текст, никакого
справочника бригад нет: строка сохраняется как есть в `players.brigade_id` и
именно по ней фильтруется лидерборд со `scope=brigade` (см. ниже).

Дубликат email → `409 {"error":"email already registered"}`.

Успех → `201`:
```json
{"player": {"id": "...", "email": "...", "username": "...", "brigade_id": "Бригада-3", "role": "user", "total_xp": 0, "wagon_progress": 0, ...}}
```

Токены не выдаются — новый игрок логинится сам через `POST /auth/login`.
Роль у создаваемого аккаунта всегда `user`; выдать роль `admin` через этот
эндпоинт нельзя (только `bootstrap-admin`).

### `POST /admin/users` — legacy upsert внешнего пользователя

Отдельный, более старый механизм для внешней (HR/SSO) интеграции — учётки
без пароля, идентифицируемые парой `(source_system, external_user_id)`,
идемпотентно. Не путать с `/admin/players`: у этого пути нет пароля и нет
входа через `/auth/login`.

```json
{
  "source_system": "hr_system",
  "external_user_id": "emp-12345",
  "display_name": "Иванов И.И.",
  "depot_id": "депо-1",
  "brigade_id": "Бригада-3",
  "assigned_class_ids": ["standard"]
}
```
Повторный вызов с тем же `(source_system, external_user_id)` обновляет
профиль, а не создаёт второй. Ответ: `{"user_id", "created": bool, "player"}`.

### `GET /admin/users/{id}/learning-summary` — сводка по игроку

`{id}` — `player.id` (UUID), не email. Возвращает:

- `subject` — кто это (`user_id`, внешние идентификаторы, `display_name`, назначенные классы).
- `session_outcomes` — по **утверждённым** (`validation_status=approved`) и завершённым сменам старого диалогового потока: `approved_completed_count`, `approved_passed_count`, и список `recent_assessments` (по каждой смене: `session_pass`, средние `loyalty`/`session_safety_score`, `critical_violations`, `unresolved_commitments`).
- `competencies` — массив `domain.CompetencyAssessment` (score/confidence/status по каждой компетенции); подсчитывается только по одобренным сменам.
- **`wagon_progression`** — прогресс по вагонным уровням (новое):
  ```json
  {
    "current_progress": 1,
    "levels": [
      {"level_id": "orientation", "order": 1, "title": "Ориентация в вагоне", "status": "passed", "attempts": 2, "passed": true, "last_attempt_at": "2026-09-27T10:00:00Z"},
      {"level_id": "service_basics", "order": 2, "title": "Сервисные просьбы", "status": "unlocked", "attempts": 1, "passed": false, "last_attempt_at": "2026-09-27T11:00:00Z"},
      {"level_id": "safety_and_conflict", "order": 3, "title": "Безопасность и конфликты", "status": "locked", "attempts": 0, "passed": false}
    ]
  }
  ```
  Одна запись на **каждый** уровень из контента (даже никогда не запускавшийся,
  с `attempts: 0`) — `status` = `locked`/`unlocked`/`passed` по правилу
  "пройди предыдущий без единого `fail`-исхода среди ситуаций смены". Считаются
  только **завершённые** (`status=finished`) вагонные смены — активная
  незакрытая смена не попадает в `attempts`.
- `provenance` — версии сценариев/правил скоринга и `excluded_draft_count`
  (сколько смен не вошло в статистику, потому что не approved/не finished).

Неизвестный `{id}` → `404`.

### `POST /admin/sessions/{id}/approve` — утвердить смену

Переводит завершённую смену старого диалогового потока в
`validation_status=approved`, но только если **все** её ситуации ссылаются на
сценарии с `validation_status=approved` в каталоге — иначе `409
{"error":"session contains unapproved content"}`. Смена не в статусе
`finished` → `409 {"error":"session is not finished"}`. Не найдена → `404`.

## Лидерборд по бригаде

Отдельный эндпоинт, не под `/admin/*`, но напрямую завязан на
`brigade_name`, указанный при создании игрока:

```
GET /api/leaderboards?scope=brigade&group_id=Бригада-3
```
Возвращает `group_size`, и на каждого игрока `rank`, `points`, `percentile`
(перцентиль считается внутри группы, не по всей компании). `scope` может
быть `company` (все), `depot` или `brigade`. Обычный `GET /api/leaderboard`
(без scope) — это тот же расчёт с `scope=company`, тоже с `percentile`.
