# VSM-400 Conductor Trainer — API Contract

Полный контракт **реально реализованного** backend-API (по коду на 26.09.2026).
Не путать с `VSM_API_DATA_CONTRACT.md` — тот файл является проектным handoff'ом
желаемого контракта; здесь — фактические маршруты, поля и формы ответов.

- Base URL: `http://localhost:8088` (docker) / `http://localhost:8080` (локально)
- Формат: `application/json; charset=utf-8`
- Аутентификация: заголовок `Authorization: Bearer <access_token>`
- Роли: `user` (по умолчанию), `admin` (создаётся локальной командой `bootstrap-admin`)

---

## 1. Аутентификация

На `/auth/*` действует rate-limit: 10 запросов в минуту на адрес подключения.

### POST `/auth/register`

Запрос:

```json
{ "email": "user@example.com", "username": "Иван", "password": "secret123" }
```

Ответ `201`:

```json
{
  "player": { "id": "...", "email": "...", "username": "...", "role": "user", "total_xp": 0, "created_at": "..." },
  "tokens": { "access_token": "...", "refresh_token": "...", "expires_in": 86400 }
}
```

Ошибки: `400` (невалидные данные), `409` (email занят).

### POST `/auth/login`

Запрос: `{ "email": "...", "password": "..." }`. Ответ `200` — та же форма, что у register.
Ошибки: `400`, `401`.

### POST `/auth/refresh`

Запрос: `{ "refresh_token": "..." }`. Ответ `200` — новая пара токенов (старый refresh отзывается).
Ошибки: `400`, `401`.

---

## 2. Общие типы

### Player

```json
{
  "id": "uuid",
  "email": "user@example.com",
  "username": "Иван",
  "role": "user",                    // user | admin
  "display_name": "Иван Иванов",     // nullable, только для внешних пользователей
  "source_system": "hr",             // nullable
  "external_user_id": "emp-123",     // nullable
  "assigned_class_ids": ["class-a"], // массив строк
  "depot_id": "depot-1",             // nullable
  "brigade_id": "brigade-1",         // nullable
  "total_xp": 140,
  "created_at": "2026-09-26T15:00:00Z"
}
```

Поля `display_name`, `source_system`, `external_user_id`, `assigned_class_ids`,
`depot_id`, `brigade_id` присутствуют только у пользователей, заведённых через
`POST /admin/users`; у игровых аккаунтов они `null`/пустые.

### Session

```json
{
  "id": "uuid",
  "player_id": "uuid",
  "status": "active",                // active | finished
  "pending_situations": ["scenario_id", "..."],
  "validation_status": "draft",      // draft | approved
  "created_at": "...",
  "finished_at": "..."               // null, пока смена не завершена
}
```

`pending_situations` — очередь scenario-id, которые ещё не разыграны (ситуации
появляются последовательно, см. §5).

### Situation (DTO)

```json
{
  "id": "uuid",
  "status": "active",                // active | closed
  "situation_def_id": "medical_asthma",
  "passenger_id": "elderly_anxious",
  "code": "medical_asthma",          // id сценария
  "name": "elderly_anxious",         // id пассажира
  "language": "ru",                  // ru | en
  "scenario": "Пассажир задыхается", // заголовок сценария
  "content_validation_status": "draft", // draft | approved; blocked не запускается
  "opening": "Пассажир хватается за грудь...",
  "loyalty": 50,
  "safety": 50,
  "timer_deadline": "2026-09-26T15:01:00Z",
  "outcome": "success",              // только после закрытия: success | partial | fail | timeout | unfinished
  "escalations": ["train_chief"],
  "xp": 20,                          // XP ситуации, после закрытия
  "remarks": [ { "code": "fast", "xp": 15, "safety": 0, "loyalty": 5, "message": "Быстрое решение" } ],
  "score_result": { "outcome": "success", "tone": "empathic", "conveyed": ["A","B"], "missed": [], "xp": 20, "safety": 65, "loyalty": 55, "remarks": [...] },
  "tone": "empathic",                // empathic | neutral | rude
  "conveyed": ["A","B"],
  "missed": []
}
```

Поля `outcome`, `xp`, `remarks`, `score_result`, `tone`, `conveyed`, `missed`
появляются после закрытия ситуации. `loyalty`/`safety` обновляются только при
закрытии (в диалоге стоят 50/50).

### Message

```json
{
  "id": 12,
  "situation_id": "uuid",
  "role": "player",                  // player | passenger | system
  "content": "Я вызову врача",
  "category": null,                  // зарезервировано, сейчас null
  "input_mode": "voice",             // nullable: text | voice
  "created_at": "..."
}
```

---

## 3. Профиль и рейтинг

### GET `/api/profile`

```json
{
  "player": { "...": "см. Player" },
  "level": 2,
  "competencies": [
    {
      "competency_id": 1,
      "code": "safety",
      "name": "Безопасность",
      "score": 60,              // накопленный XP, null если нет evidence
      "confidence": 3,          // число contributing-ситуаций
      "status": "provisional",  // insufficient | provisional | assessed
      "evidence_count": 3
    }
  ],
  "achievements": []
}
```

`level = total_xp/100 + 1` (прототипная кривая). Компетенции соответствуют
типам ситуаций: `service`, `conflict`, `medical`, `safety`, `informational`
(плюс служебные `empathy`, `communication` из ранних версий).

### GET `/api/leaderboard`

```json
{
  "leaderboard": [
    { "rank": 1, "player_id": "uuid", "username": "Иван", "total_xp": 420 }
  ]
}
```

### GET `/api/leaderboards?scope=company|depot|brigade&group_id=...`

`scope` по умолчанию `company`. Для `depot`/`brigade` фильтр идёт по
`players.depot_id`/`players.brigade_id` (задаются через `POST /admin/users`).

```json
{
  "group_scope": "depot",
  "group_id": "depot-1",
  "group_size": 5,
  "entries": [
    { "rank": 1, "player_id": "uuid", "username": "Иван", "leaderboard_points_total": 420, "percentile": 100 }
  ]
}
```

---

## 4. Смена (сессия)

### Демонстрационная смена с ветвлением

`POST /api/session/simulations` создаёт новую смену в `POINTS_NAMESPACE=demo`.
`GET /api/session/simulations/{id}` возвращает её текущее состояние. Ответ:

```json
{
  "run": {
    "id": "uuid", "scenario_id": "demo_service_branch",
    "scenario_version": "1.0.0", "content_validation_status": "draft",
    "state_version": 0, "status": "active", "loyalty": 80, "safety": 100,
    "path": []
  },
  "event": {
    "id": "service_request", "text": "Пассажир просит услугу...",
    "choices": [{"id": "check_availability", "text": "Проверить наличие прежде чем обещать"}]
  }
}
```

`POST /api/session/simulations/{id}/actions` принимает
`{"command_id":"uuid","expected_state_version":0,"choice_id":"check_availability"}`.
Ответ `200` имеет ту же форму с новой версией и следующим событием. Повтор того
же `command_id` возвращает прежний результат; новая команда со старой версией
получает `409`. Невозможный выбор получает `400`, чужая смена — `404`.
Правила выбора и внутренние флаги остаются на сервере. Конфигурация сценария
закрепляется в записи смены, поэтому изменение файла не меняет уже начатый рейс.
Это пока отдельный демонстрационный путь: движение, параллельные события и
полный debrief ещё не реализованы.

### POST `/api/session/start`

Создаёт смену и **первую** ситуацию; остальные кладёт в `pending_situations`.
В `POINTS_NAMESPACE=demo` допускаются `draft` и `approved` сценарии. В других
пространствах доступны только `approved`; если их нет, ответ `409`.
Ответ `201`:

```json
{
  "session": { "...": "см. Session" },
  "situations": [ { "...": "см. Situation DTO" } ]
}
```

### GET `/api/session/{id}`

Состояние смены. Ответ `200` — та же форма, что и у `start`.

### POST `/api/session/{id}/finish`

Закрывает оставшиеся активные ситуации, считает разбор и начисляет XP.
Идемпотентен (повторный вызов не удваивает XP).

```json
{
  "session_id": "uuid",
  "scenario_version": "1.0.0",
  "scoring_rule_version": "points-v1",
  "validation_status": "draft",
  "points_namespace": "demo",
  "total_xp": 120,
  "session_pass": true,
  "world_safety_current": 65,
  "session_safety_score": 65,
  "loyalty": 55,
  "critical_violations": 0,
  "unresolved_commitments": 0,
  "competencies_xp": { "safety": 40, "service": 10 },
  "leaderboard_points_delta": 120,
  "leaderboard_points_total": 260,
  "leaderboard_eligible": false,
  "situations": [
    {
      "situation_id": "uuid",
      "code": "medical_asthma",
      "name": "elderly_anxious",
      "outcome": "success",
      "loyalty": 55,
      "safety": 65,
      "xp": 20,
      "remarks": [ { "code": "fast", "xp": 15, "safety": 0, "loyalty": 5, "message": "Быстрое решение" } ],
      "score_result": { "...": "полный ScoreSummary" }
    }
  ]
}
```

Семантика полей результата:

- `session_pass` — `true`, если нет ситуаций с outcome `fail`/`timeout`/`unfinished`.
- `critical_violations` — число ситуаций с outcome `fail` или `timeout`.
- `unresolved_commitments` — число ситуаций с outcome `unfinished`.
- `session_safety_score` — среднее финальных `safety` по ситуациям.
- `world_safety_current` — в текущей модели равно `session_safety_score` (шкалы
  считаются при закрытии, живого состояния мира нет).
- `leaderboard_points_delta` = `total_xp`; `leaderboard_points_total` — накопленный
  `total_xp` игрока после начисления. `leaderboard_eligible` — `true` только при
  `POINTS_NAMESPACE=official`.

---

## 5. Ситуация (диалог)

### GET `/api/situation/{id}`

```json
{
  "situation": { "...": "см. Situation DTO" },
  "messages": [ { "...": "см. Message" } ]
}
```

### POST `/api/situation/{id}/message`

Запрос:

```json
{ "text": "Не переживайте, я вызову врача.", "input_mode": "voice" }
```

`input_mode` опционально: `text` | `voice`. Ответ `200`:

```json
{
  "situation_id": "uuid",
  "reply": "Спасибо, буду признателен...",
  "loyalty": 50,
  "safety": 50,
  "status": "active",
  "outcome": null,
  "timer_deadline": "...",
  "closed": false,
  "turn_count": 2
}
```

Если в момент отправки таймер истёк — ситуация закрывается, `closed: true`,
`outcome` заполняется. Ошибки: `404`, `409` (закрыта/смена завершена).

### POST `/api/situation/{id}/escalate`

Запрос: `{ "to": "medic" }`. Допустимые адресаты: `train_chief`, `ptb`, `police`,
`medic`, `ambulance`. Ответ `200`:

```json
{ "situation_id": "uuid", "escalations": ["medic"] }
```

### POST `/api/situation/{id}/finish`

Закрывает ситуацию и запускает скоринг (один LLM-вызов). Ответ `200`:

```json
{
  "situation_id": "uuid",
  "outcome": "partial",
  "tone": "empathic",
  "conveyed": ["A","B"],
  "missed": ["C"],
  "xp": 20,
  "safety": 65,
  "loyalty": 55,
  "remarks": [
    { "code": "fast", "xp": 15, "safety": 0, "loyalty": 5, "message": "Быстрое решение" }
  ]
}
```

---

## 6. Админ / HR

Требуют роль `admin` (мидлварь `AdminAuth`). Не-админ получает `403`.

### POST `/admin/users`

Регистрация внешнего сотрудника (идемпотентно по `source_system` + `external_user_id`).

```json
{
  "source_system": "hr",
  "external_user_id": "emp-123",
  "assigned_class_ids": ["class-a", "class-b"],
  "command_id": "cmd-0007",
  "display_name": "Иван Иванов",
  "depot_id": "depot-1",
  "brigade_id": "brigade-1"
}
```

Ответ `200`:

```json
{
  "user_id": "uuid",
  "created": true,
  "player": { "...": "см. Player" }
}
```

`created=false`, если сотрудник уже был заведён (запись обновлена).
Отсутствующий `assigned_class_ids` сохраняется как пустой массив.

### GET `/admin/users/{id}/learning-summary`

```json
{
  "data_status": "available",
  "subject": {
    "user_id": "uuid",
    "source_system": "hr",
    "external_user_id": "emp-123",
    "assigned_class_ids": ["class-a"],
    "display_name": "Иван Иванов"
  },
  "training_scope": {
    "class_ids": ["class-a"],
    "mastery_stage": null,
    "validation_status": "approved",
    "scenario_version": "1.0.0",
    "scoring_rule_version": "points-v1",
    "assessed_at": "2026-09-26T16:00:00Z"
  },
  "session_outcomes": {
    "approved_completed_count": 3,
    "approved_passed_count": 2,
    "recent_assessments": [
      {
        "session_id": "uuid",
        "completed_at": "2026-09-26T16:00:00Z",
        "session_pass": true,
        "world_safety_current": 65,
        "session_safety_score": 65,
        "loyalty": 55,
        "critical_violations": 0,
        "unresolved_commitments": 0
      }
    ]
  },
  "competencies": [ { "...": "см. profile competencies" } ],
  "provenance": {
    "as_of": "2026-09-26T16:30:00Z",
    "scenario_version": "1.0.0",
    "scoring_rule_version": "points-v1",
    "excluded_draft_count": 2
  }
}
```

В `session_outcomes` и `competencies` учитываются **только завершённые approved**-сессии;
остальные попадают в `provenance.excluded_draft_count`. Если утверждённых
сессий нет, `data_status` равен `no_approved_data`, а оценки компетенций
не содержат evidence.
Неотыгранные `pending_situations` учитываются в `unresolved_commitments` и
исключают `session_pass=true`.

### POST `/admin/sessions/{id}/approve`

Помечает завершённую сессию `validation_status = approved`. Активная сессия
возвращает `409`, несуществующая — `404`. Сессия с `draft`/`blocked` контентом
возвращает `409` и не становится утверждённым результатом. Ответ `200`:

```json
{ "session_id": "uuid", "validation_status": "approved" }
```

---

## 7. Health

- `GET /healthz` — `ok` (liveness).
- `GET /readyz` — `ok` или `503` при недоступной БД.
- `GET /metrics` — `{ "llm_errors": 0 }`.

---

## 8. Справочники (enums)

| Поле | Значения |
|---|---|
| `role` | `user`, `admin` |
| `session.status` | `active`, `finished` |
| `session.validation_status` | `draft`, `approved` |
| `situation.status` | `active`, `closed` |
| `outcome` | `success`, `partial`, `fail`, `timeout`, `unfinished` |
| `tone` | `empathic`, `neutral`, `rude` |
| `message.role` | `player`, `passenger`, `system` |
| `message.input_mode` | `text`, `voice` |
| escalation target | `train_chief`, `ptb`, `police`, `medic`, `ambulance` |
| leaderboard scope | `company`, `depot`, `brigade` |
| competency status | `insufficient`, `provisional`, `assessed` |
| remark codes | `fast`, `on_time`, `timeout`, `escalation_ok`, `no_escalation`, `false_escalation`, `wrong_target`, `empathic`, `rude`, `missed_point`, `solved` |

---

## 9. Ошибки

Единая форма: `{ "error": "описание" }`.

| Код | Значение |
|---|---|
| `400` | Невалидный запрос / id / enum |
| `401` | Отсутствует/невалидный токен |
| `403` | Требуется роль `admin` |
| `404` | Ресурс не найден |
| `409` | Конфликт (ситуация/сессия закрыта, email занят, stale state) |
| `429` | Rate-limit на `/auth/*` |
| `500` | Внутренняя ошибка |

---

## 10. Переменные окружения (влияют на контракт)

| Переменная | По умолчанию | Влияние |
|---|---|---|
| `ADMIN_BOOTSTRAP_EMAIL`, `ADMIN_BOOTSTRAP_PASSWORD` | — | используются только локальной командой `bootstrap-admin`; регистрация через API всегда создаёт `user` |
| `POINTS_NAMESPACE` | `demo` | `leaderboard_eligible` (`true` только при `official`) |
| `SITUATIONS_PER_SESSION` | `4` | число ситуаций в смене |
| `SITUATION_TIMEOUT` | — | (устарело, таймер берётся из сценария `time_limit_sec`) |

---

## 11. Примечания к модели

1. **Ситуации генерируются последовательно**: `start` создаёт одну ситуацию,
   следующая появляется после закрытия предыдущей (очередь в
   `session.pending_situations`). Фронту нужно перезапрашивать `GET /session/{id}`
   после закрытия ситуации.
2. **Скоринг один раз при закрытии**: LLM отвечает фактами (`conveyed/missed/tone/
   escalation`), исход и баллы считает код детерминированно.
3. **Две шкалы** (`loyalty`/`safety`) стартуют 50/50 и фиксируются при закрытии.
4. **Компетенции** соответствуют типам ситуаций; `score` — накопленный XP (может
   быть отрицательным), `status` — `insufficient`/`provisional`.
5. **Рейтинг** = накопленный `total_xp` (правило GDD v1.0).
