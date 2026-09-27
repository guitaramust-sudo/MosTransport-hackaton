# API тренажёра ВСМ

Описание реализованного API для сдачи проекта. Сверено с маршрутизатором, обработчиками и моделями backend на 28.09.2026. Это документация фактического прототипа, а не обещание будущих функций. Основной источник маршрутов — [`backend/cmd/server/main.go`](../backend/cmd/server/main.go); структуры ответов — [`backend/internal/handler`](../backend/internal/handler) и [`backend/internal/service`](../backend/internal/service).

## Адрес, авторизация и общие правила

- При запуске через корневой `docker compose`: `http://localhost:8088`. Пути ниже указаны относительно этого адреса. Внутри контейнера backend слушает порт `8080`.
- JSON-запросы отправляются с `Content-Type: application/json`. Неизвестные поля в JSON тела обработчики отвергают. Даты и время в ответах сериализуются как RFC 3339; идентификаторы сессий и игроков — UUID.
- `POST /auth/login` и `POST /auth/refresh` открыты. Все HTTP-пути `/api/*` требуют `Authorization: Bearer <access_token>`; `/admin/*` дополнительно требуют роль `admin`. Публичной регистрации нет: аккаунт игрока создаёт администратор.
- WebSocket `GET /api/wagon/{id}/ws?token=<access_token>` проверяет JWT из параметра `token`, владельца сессии и её активный статус. Токен в URL может попасть в журналы доступа; клиенту следует использовать только защищённое соединение вне локального демо.
- Обновление refresh token выдаёт новую пару и расходует предыдущий refresh token. Для `/auth/*` действует общий лимит 10 запросов в минуту на IP; при `429` возвращается `Retry-After: 6`.
- Типичная ошибка: `{"error":"описание"}`. В отдельных конфликтах вагонного режима возвращается `{"status":"locked"}` или `{"status":"coming_soon"}`. `400` — неверные данные, `401` — нет/истёк токен, `403` — нет роли администратора, `404` — объект не найден или не принадлежит игроку, `409` — конфликт состояния, `429` — лимит авторизации, `500` — внутренняя ошибка. Конкретный набор статусов зависит от маршрута.

## Служебные маршруты

| Метод и путь | Авторизация | Ответ |
|---|---|---|
| `GET /healthz` | Нет | `200`, текст `ok`: процесс отвечает. |
| `GET /readyz` | Нет | `200`, текст `ok`, если доступна БД; иначе `503`, текст `database unavailable`. |
| `GET /metrics` | Нет | `200`, JSON `{"llm_errors":0}`: счётчик ошибок LLM, а не полный набор бизнес-метрик. |

## Авторизация и учётные записи

| Метод и путь | Роль | Тело запроса / результат |
|---|---|---|
| `POST /auth/login` | Любой | `{ "email": string, "password": string }` → `200`, объект `AuthResult`. Неверные данные: `401`. |
| `POST /auth/refresh` | Любой | `{ "refresh_token": string }` → `200`, новый `AuthResult`. Негодный или уже использованный токен: `401`. |
| `POST /admin/players` | `admin` | `{ "email": string, "username": string, "password": string, "brigade_name": string }` → `201 { "player": Player }`. Пароль минимум 6 символов; совпадение email: `409`. Токены нового игрока не возвращаются. |
| `POST /admin/users` | `admin` | Upsert внешней записи: обязательны `source_system`, `external_user_id`; допустимы `assigned_class_ids`, `display_name`, `depot_id`, `brigade_id`, `command_id`. → `200 { "user_id": UUID, "created": boolean, "player": Player }`. Идемпотентность по паре `source_system` + `external_user_id`; `command_id` пока не участвует в обработке. Это интеграционный путь без пароля для обычного логина. |

`AuthResult` содержит `player` и `tokens`:

```json
{
  "player": {
    "id": "00000000-0000-0000-0000-000000000001",
    "email": "player@example.test",
    "username": "player",
    "role": "user",
    "total_xp": 0,
    "wagon_progress": 0,
    "created_at": "2026-09-28T00:00:00Z"
  },
  "tokens": {
    "access_token": "<JWT>",
    "refresh_token": "<opaque-token>",
    "expires_in": 86400
  }
}
```

Число `expires_in` зависит от настройки `JWT_ACCESS_TTL`; значение выше — пример. `Player` может также содержать `display_name`, `source_system`, `external_user_id`, `assigned_class_ids`, `depot_id`, `brigade_id`. Хеш пароля в JSON не выдаётся.

## Профиль, рейтинг и уведомления

| Метод и путь | Роль | Результат |
|---|---|---|
| `GET /api/profile` | Игрок/JWT | `{player, level, competencies, achievements, leaderboard_points_total}`. `competencies[]`: `competency_id`, `code`, `name`, `score` (`null`, если нет свидетельств), `confidence`, `status`, `evidence_count`. |
| `GET /api/leaderboard` | Игрок/JWT | `{ "leaderboard": [{rank, player_id, username, leaderboard_points_total, percentile}, ...] }`; первые 10 игроков всей компании. |
| `GET /api/leaderboards?scope=company\|depot\|brigade&group_id=...` | Игрок/JWT | `{group_scope, group_id, group_size, entries:[{rank, player_id, username, leaderboard_points_total, percentile}]}`; максимум 50 записей. `scope` по умолчанию `company`; некорректный `scope` → `400`. |
| `GET /api/notifications` | Игрок/JWT | `{ "notifications": Notification[] }` — внутренние уведомления ветвящейся демо-симуляции, не статус доставки мобильного push. |
| `GET /api/challenges/weekly` | Игрок/JWT | `{challenge_id, seed_variants, target, completed, reward_xp}` — прогресс демонстрационного недельного задания. |
| `POST /api/me/push-subscriptions` | Игрок/JWT | `{ "platform": string, "device_token": string }` → `201 {"status":"registered"}`. Клиент передаёт `android` или `ios`; сервер проверяет лишь непустые поля. Токен сохраняется за текущим игроком; фактическая отправка сейчас логируется, см. ограничения ниже. |

`Notification` содержит `id`, `type`, `subject_key`, `payload`, `created_at`. Рейтинг строится **по накопленным очкам** в текущем `POINTS_NAMESPACE`. Он не равен XP и не равен призовому балансу. `level` в профиле — прототипная шкала XP, не кадровый разряд. Компетенции с `status: "provisional"` отражают игровые свидетельства; сами по себе они не подтверждают профессиональную квалификацию.

## Учебный маршрут: глава → урок → практика

| Метод и путь | Роль | Тело запроса / результат |
|---|---|---|
| `GET /api/learning/map` | Игрок/JWT | `{chapters:[{chapter_id, title, order, lessons:[{lesson_id, title, order, status, badge_id}]}]}`. `status`: `locked`, `unlocked`, `completed`. Пустые главы возвращаются с `lessons: []`. |
| `GET /api/learning/lessons/{id}` | Игрок/JWT | Детали открытого урока: `lesson_id`, `title`, `theory_cards`, `questions`, `progress`, `completion_rule`, `required_anchor_ids`, `required_object_ids`, `estimated_min`, `badge_id`. Неизвестный урок → `404`, закрытый → `409`. |
| `POST /api/learning/lessons/{id}/answers` | Игрок/JWT | `{ "question_id": string, "option_id": string }` → `{correct, feedback, phase_pass}`. Вопрос вне урока → `400`. |
| `POST /api/learning/lessons/{id}/practice` | Игрок/JWT | `{}` → `201 { "session_id": UUID, "ws_path": "/api/wagon/{session_id}/ws" }`. Если теория не пройдена → `409`. Практика проходит через WebSocket вагона. |
| `POST /api/learning/lessons/{id}/finalize` | Игрок/JWT | `{}` → `FinalizeResult` с разбором, недостающими условиями и, при полном успехе, наградой. Без начатой практики → `409`. |
| `GET /api/me/learning` | Игрок/JWT | `{lessons:[{lesson_id,status,completed_at?}], prize_balance, prize_next_expiry, prize_shirt_threshold, prize_shirt_progress}`; последний показатель — доля порога от `0` до `1`. |

Вопрос урока выдаётся как `{question_id, phase, type, prompt, options:[{option_id,text}]}`. Ключ правильного ответа и таблица обратной связи клиенту не отправляются. `progress` включает `theory_pass`, `practice_session_id`, `practice_pass`, `practice_pass_count`, `practice_check_pass`, `completed_at`, `content_version`. Для допуска к практике все теоретические вопросы должны иметь хотя бы один правильный ответ. Для завершения урока нужны проверочные вопросы и **две разные успешно пройденные практические сессии**; повторный `finalize` одной и той же сессии счётчик не увеличивает. Практику сначала завершают через `POST /api/session/{id}/finish`. В B01 проверяются посещённые точки и осмотренные объекты; в разговорном B02 требуются закрытая ситуация с успешным результатом и хотя бы одно сообщение игрока. `finalize` сообщает, какие условия ещё не выполнены.

`FinalizeResult`:

```json
{
  "completed": true,
  "practice_pass_count": 2,
  "practice_pass_required": 2,
  "award_granted": true,
  "xp_awarded": 20,
  "badge_id": "<badge-id>",
  "debrief": "<текст разбора>",
  "found_anchors": [],
  "missing_anchors": [],
  "found_objects": [],
  "missing_objects": []
}
```

При незавершённом уроке `completed: false`, а `missing` содержит одно или несколько значений `theory_pass`, `practice_pass`, `practice_check_pass`, `practice_runs`; для сценарного урока отдельно может быть `scenario_pass`. `award_granted: false` при повторной выдаче: XP и бейдж за урок начисляются один раз. За первое завершение B01/B02 начисляется отдельная запись **+10 призовых очков**, срок каждой записи — **6 × 24 часа** от начисления. Порог футболки `50` — демонстрационный индикатор, выдача реального приза через API не реализована.

## Вагонная смена и WebSocket

| Метод и путь | Роль | Тело запроса / результат |
|---|---|---|
| `GET /api/wagon/classes` | Игрок/JWT | `{ "classes": {"first":"available","standard":"available","comfort":"coming_soon","business":"coming_soon"} }` при текущем конфиге. Это статус технической конфигурации, а не готовности полного учебного курса класса. |
| `GET /api/wagon/levels` | Игрок/JWT | `{ "levels": [{id, order, title, intro?, status}] }`; `status`: `locked`, `unlocked`, `passed`. |
| `POST /api/session/wagon/start` | Игрок/JWT | `{ "level_id": string }` → `201 {session_id, ws_path}`. Неизвестный уровень → `404`; закрытый → `409 {"status":"locked"}`; класс без реализации → `409 {"status":"coming_soon"}`. |
| `GET /api/wagon/{id}/ws?token=<access_token>` | Владелец активной сессии | WebSocket upgrade (`101`). Неверный UUID → `400`; токен → `401`; чужая/отсутствующая сессия → `404`; завершённая → `409`. |

После подключения сервер отправляет снимок:

```json
{
  "type": "state",
  "game_time_s": 12,
  "wagon_state": {
    "class_id": "first",
    "level_id": "",
    "restricted_anchors": ["cab_entrance_boundary"],
    "seats": [],
    "player": {"at": "service_zone"},
    "carried_items": [],
    "started_at": "2026-09-28T00:00:00Z",
    "duration_s": 480
  },
  "active_situations": []
}
```

`wagon_state.seats[]` содержит `anchor`, `passenger_def_id`, `situation_id?`, `situation_def_id?`, `restricted_reached?`, `actor`. Позиция актора — `{at, moving?}`; `moving`, если есть, содержит `from`, `to`, `started_at`, `duration_s`. Снимок также передаёт `visited_anchors?`, `inspected_objects?`. `active_situations[]`: `situation_id`, `seat_anchor`, `type`, `pool`. Для разговора и результата ситуации используются HTTP-маршруты ниже.

Клиент отправляет JSON-команды:

```json
{"type":"move_to","anchor":"seat_1"}
{"type":"visit","anchor":"sanitary_zone"}
{"type":"inspect","item":"<object-id>"}
{"type":"pick_item","item":"<item-id>"}
{"type":"give_item","item":"<item-id>","situation_id":"<UUID>"}
{"type":"redirect","situation_id":"<UUID>"}
```

Ответ на удачную команду — новый `state`; при ошибке — `{"type":"error","message":"..."}`. Неизвестные команды не поддерживаются. Идентификаторы `anchor` и `item` должны соответствовать данным вагона и сценария; сервер проверяет перемещения, доступ к служебным зонам и физические действия. В одном WebSocket подключении допускается один активный клиент на сессию: новое подключение заменяет прежнее.

## Последовательная диалоговая смена и ситуации

Это отдельный поток от ветвящейся демо-симуляции. Уроки также создают вагонную сессию, после которой используют те же маршруты ситуации и завершения сессии.

| Метод и путь | Роль | Тело запроса / результат |
|---|---|---|
| `POST /api/session/start` | Игрок/JWT | `{}` → `201 {session: Session, situations: Situation[]}`. Нет подходящих сценариев → `409`. |
| `GET /api/session/{id}` | Владелец/JWT | `{session: Session, situations: Situation[]}`. У этого ответа поле `opening` у ситуаций не заполняется; начальную реплику получают через `GET /api/situation/{id}`. |
| `POST /api/session/{id}/finish` | Владелец/JWT | `{}` → `Breakdown`. Завершает оставшиеся ситуации, рассчитывает результат и награды; повторное завершение не начисляет XP повторно. |
| `GET /api/situation/{id}` | Владелец/JWT | `{situation: Situation, messages: Message[]}`; `Message` содержит `id`, `situation_id`, `role`, `content`, `category?`, `input_mode?`, `created_at`. |
| `POST /api/situation/{id}/message` | Владелец/JWT | `{ "text": string, "input_mode"?: "text"\|"voice" }` → `{situation_id, reply, loyalty, safety, status, outcome?, timer_deadline, closed, turn_count}`. `voice` фиксирует источник текста, аудиофайл этот API не принимает. |
| `POST /api/situation/{id}/escalate` | Владелец/JWT | `{ "to": string }` → `{situation_id, escalations:[string]}`; недопустимый адресат → `400`. |
| `POST /api/situation/{id}/finish` | Владелец/JWT | `{}` → `{situation_id, outcome, tone, conveyed, missed, xp, safety, loyalty, remarks}`. Закрытая ситуация → `409`. |

`Session` содержит `id`, `player_id`, `status` (`active`/`finished`), `validation_status`, `created_at`, `finished_at`, а для вагонного режима — `wagon_state`. `Situation` содержит `id`, `status`, код/название/сценарий пассажира, шкалы, таймер, физическое требование и его выполнение, результат, XP, эскалации и замечания. В начальном ответе `/session/start` может присутствовать `opening`.

`Breakdown` включает `session_id`, `scenario_version`, `scoring_rule_version`, `validation_status`, `points_namespace`, `total_xp`, `session_pass`, `world_safety_current`, `session_safety_score`, `loyalty`, `critical_violations`, `unresolved_commitments`, `competencies_xp`, `leaderboard_points_delta`, `leaderboard_points_total`, `leaderboard_eligible`, `situations[]`. У каждого элемента `situations[]`: `situation_id`, `code`, `name`, `outcome`, `loyalty`, `safety`, `xp`, `remarks?`, `score_result?`.

## Отдельная ветвящаяся демо-симуляция

Этот HTTP-поток работает без WebSocket и возвращает `SimulationView` после каждого действия. Он не является учебным уроком B01/B02.

| Метод и путь | Роль | Тело запроса / результат |
|---|---|---|
| `POST /api/session/simulations` | Игрок/JWT | `{}` → `201 SimulationView`. Доступность зависит от namespace и статуса контента; отказ → `409`. |
| `GET /api/session/simulations/{id}` | Владелец/JWT | `SimulationView`; неизвестный или чужой запуск → `404`. |
| `POST /api/session/simulations/{id}/actions` | Владелец/JWT | `{command_id: UUID, expected_state_version: integer, action_id?: string, event_id?: string, target?: string, choice_id?: string}` → `SimulationView`. Нужен `action_id` или `choice_id`. |
| `POST /api/session/simulations/{id}/dialogue` | Владелец/JWT | `{command_id: UUID, expected_state_version: integer, event_id?: string, text: string}` → `SimulationView`; текст 1–600 символов, без `event_id` используется текущий. |
| `GET /api/session/simulations/{id}/result` | Владелец/JWT | `SimulationResult` после завершения; активный запуск → `409`. |

`SimulationView` содержит `run`, `event?`, `events`, `observable_cues`, `passenger`, `dialogue`. В `run` есть `id`, `scenario_id`, `scenario_version`, `content_validation_status`, **`state_version`**, `status`, `loyalty`, `safety`, `path`, `location`, `game_time_s`, `seed_variant`, `deadline_at?`, `timed_out`. `events[]` содержит `id`, `text`, `location`, `choices:[{id,text}]`. Для следующей команды клиент берёт актуальный `state_version`, а `command_id` создаёт как новый UUID; повтор одного `command_id` защищает от двойного применения команды. Устаревшая версия состояния → `409 {"error":"state version changed"}`.

`SimulationResult` содержит `session_id`, версии сценария и скоринга, `validation_status`, `completed_at`, шкалы, `timed_out`, `session_pass`, `action_log_hash`, `debrief[]`, `points_namespace`, `leaderboard_points_delta`, `leaderboard_points_total`, `leaderboard_eligible`. Разбор `debrief[]` включает действия, изменение шкал, объяснение, варианты лучших решений и при наличии текст диалога.

## API администратора и HR-сводка

| Метод и путь | Роль | Результат |
|---|---|---|
| `GET /admin/users/{id}/learning-summary` | `admin` | `LearningSummary` по UUID игрока; неизвестный пользователь → `404`. |
| `POST /admin/sessions/{id}/approve` | `admin` | `{}` → `200 {"session_id":UUID,"validation_status":"approved"}`. Нужна завершённая сессия с утверждённым контентом; иначе `409`. |
| `POST /admin/users/{id}/prize-credits/demo-seed` | `admin` | `{}` → `{ "granted": boolean }`. Тестовая запись +40 призовых очков с остатком около суток; повтор идемпотентен. Только для демонстрации механики истечения. |

Схема `LearningSummary`:

| Поле | Содержимое |
|---|---|
| `data_status` | `available` либо `no_approved_data`. |
| `subject` | `user_id`, `source_system`, `external_user_id`, `assigned_class_ids`, `display_name`. |
| `training_scope` | `class_ids`, `mastery_stage`, `validation_status`, `scenario_version`, `scoring_rule_version`, `assessed_at`. |
| `session_outcomes` | `approved_completed_count`, `approved_passed_count`, `recent_assessments[]`; в каждой оценке: время, прохождение, Safety, Loyalty, критические нарушения и незакрытые ситуации. |
| `competencies[]` | `competency_id`, `code`, `name`, `score`, `confidence`, `status`, `evidence_count`; для HR рассчитываются по утверждённым сессиям. |
| `wagon_progression` | `current_progress` и `levels[]` со статусом, числом попыток, фактом прохождения и датой последней попытки. |
| `provenance` | `as_of`, версии сценария/скоринга, `excluded_draft_count`. |

HR-сводка исключает черновые сессии из агрегатов и явно показывает их число. Статус `no_approved_data` означает отсутствие утверждённых результатов, а не плохую работу сотрудника. API не принимает кадровое решение о переводе в другой класс вагона автоматически.

## Текущие границы интеграции

- Доступность `first` и `standard` в `/api/wagon/classes` означает наличие конфигурации вагонного режима. Полный учебный цикл в текущем контенте подготовлен для B01–B02 класса First; остальные главы карты могут не содержать уроков. Классы `comfort` и `business` помечены `coming_soon`.
- Регистрация через `/api/me/push-subscriptions` сохраняет токен устройства (`{platform,device_token}` → `201 {"status":"registered"}`). Сервер формирует события `lesson_unlocked` и `prize_credit_expiring`, но в текущей сборке отправитель `LogSender` **пишет их в журнал**; подтверждённой внешней доставки Expo/FCM этот backend не предоставляет. Для мобильного клиента дополнительно нужна development build, поскольку Expo Go на Android SDK 53 не поддерживает remote push.
- `POINTS_NAMESPACE=demo` по умолчанию. Учебный контент и многие оценки имеют статус `draft`; демонстрационные значения не следует выдавать за официальную аттестацию. Запрос `/admin/sessions/{id}/approve` требует утверждённого содержимого и не превращает черновой сценарий в утверждённый автоматически.
- Публичного маршрута для просмотра профиля **другого** игрока сейчас нет. Рейтинг отдаёт только имя, очки, место и процентиль; полная HR-сводка доступна администратору.
- Публичного маршрута для списания призовых очков или выдачи футболки нет. Музыка и визуальное оформление клиента не являются частью API.

## Короткий пример вызовов

После запуска проекта и создания игрока администратором:

```bash
curl -sS http://localhost:8088/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"player@example.test","password":"<password>"}'
```

Из поля `tokens.access_token` ответа подставьте токен в следующий запрос:

```bash
curl -sS http://localhost:8088/api/learning/map \
  -H 'Authorization: Bearer <access_token>'
```

После выбора доступного `lesson_id`: `GET /api/learning/lessons/{id}` → ответы на теоретические вопросы → `POST .../practice` → подключение к возвращённому `ws_path` → завершение сессии → `POST .../finalize` → новая успешная практика и её `finalize` → ответы на практические вопросы → окончательный `finalize` → `GET /api/me/learning`.
