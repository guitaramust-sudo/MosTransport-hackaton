# Интеграция real-time вагона во фронтенд

Документ для разработчика Expo/React Native и web-клиента. Фактический контракт
бэкенда: [WAGON_API.md](../backend/WAGON_API.md). Режим реализован в
`WagonLobbyPage` и `WagonPage`; он **отдельный** от `SimulationPage`
(последовательная смена) и `LiveSimulationPage` (ветвящийся демо-сценарий).

## Что нужно показать игроку

В классе `standard` есть шесть мест с пассажирами, сервисная точка и служебная
зона. Сервер хранит только символьные якоря `seat_1`…`seat_6`,
`service_point`, `staff_zone`. Фронтенд сопоставляет их с координатами своей
3D-сцены и рисует персонажей. Готовые координаты сервер не передаёт.

Ситуации возникают у нескольких пассажиров независимо. WebSocket присылает
состояние мест и идентификатор сценария, **без текстового объявления и без
метки приоритета**. Покажите поведение пассажира: для `cold` — мёрзнет,
`thirsty` — ищет воду, `tired` — зевает, `zone_intrusion` — встаёт и идёт к
`staff_zone`. Для остальных сценариев из каталога нужен общий визуальный
сигнал без раскрытия текста; всего каталог содержит 54 сценария. Детали и
разговор открывайте при взаимодействии игрока с пассажиром.

## Где реализовано в текущем клиенте

| Файл | Назначение |
| --- | --- |
| [src/api/client.ts](src/api/client.ts) | HTTP-методы уровней/смены, актуальный access-токен и сборка WS URL. |
| [src/types/index.ts](src/types/index.ts) | Типы вагона, уровней и физических требований. |
| [src/app/store.ts](src/app/store.ts) | ID смены, полный снимок, выбранная ситуация и статус соединения. |
| [src/pages/WagonLobbyPage.tsx](src/pages/WagonLobbyPage.tsx), [src/pages/WagonPage.tsx](src/pages/WagonPage.tsx) | Выбор уровня, запуск, диалог, физические действия и переподключение. |
| [src/components/WagonWorld.web.tsx](src/components/WagonWorld.web.tsx), [src/components/WagonWorld.native.tsx](src/components/WagonWorld.native.tsx) | 3D-сцена и символические якоря. |

Не подставляйте вагонный `session_id` в `/api/session/simulations/*`: это другой
движок с другим форматом состояния.

## HTTP и порядок запуска

Все HTTP-запросы используют `Authorization: Bearer <access_token>`.

1. `GET /api/wagon/levels` → `{"levels":[{"id":"orientation","order":1,
   "title":"Ориентация в вагоне","intro":"...","status":"unlocked"},…]}`.
   `status` бывает `locked`, `unlocked` и `passed`; закрытый уровень нельзя
   запускать, а пройденный можно повторить. `GET /api/wagon/classes` показывает
   доступность классов отдельно от уровней.
2. `POST /api/session/wagon/start` с `{"level_id":"orientation"}` → `201` и
   `{"session_id":"<uuid>","ws_path":"/api/wagon/<uuid>/ws"}`. Сохраните
   `session_id`: отдельного эндпоинта для поиска последней вагонной смены нет.
3. Подключитесь к `ws_path` по WebSocket с JWT в query-параметре `token`.
   Первый кадр — полный `state`. Смену можно восстановить через
   `GET /api/session/{session_id}` (ответ `{"session": {...}, "situations": [...]}`;
   состояние находится в `session.wagon_state`) и переподключение к тому же WS URL.
4. Для разговора с конкретным пассажиром используйте
   `GET /api/situation/{situation_id}`,
   `POST /api/situation/{situation_id}/message` с
   `{"text":"...","input_mode":"text"}` (или `"voice"` после STT),
   при необходимости `/escalate` с `{"to":"train_chief"}`, затем `/finish`.
   Ответ `/message` содержит `reply` пассажира. Физическое действие не
   заменяет разговор.
5. `POST /api/session/{session_id}/finish` закрывает оставшиеся ситуации,
   возвращает разбор и закрывает WebSocket. Покажите итог по ответу REST.

После 480 секунд новые ситуации перестают появляться. Сервер сам смену не
завершает; текущий `WagonPage` вызывает `/finish` при истечении времени.
Если игрок просто закрыл экран раньше, сервер продолжает смену и таймеры
ситуаций.

## Типы для клиента

Это форма сообщений, которую возвращает код. Пустой `carried_items` может
приходить как `[]` или `null`; нормализуйте через `?? []`.

```ts
type Anchor = string // seat_1…seat_6 | service_point | staff_zone
type WagonMove = { from: Anchor; to: Anchor; started_at: string; duration_s: number }
type WagonActor = { at: Anchor; moving?: WagonMove }
type WagonSeat = {
  anchor: Anchor
  passenger_def_id: string
  situation_id?: string
  situation_def_id?: string
  restricted_reached?: boolean
  actor: WagonActor
}
type WagonState = {
  class_id: 'standard'
  level_id: string
  restricted_anchors: Anchor[]
  seats: WagonSeat[]
  player: WagonActor
  carried_items: string[] | null
  started_at: string
  duration_s: number
}
type WagonSnapshot = {
  type: 'state'
  game_time_s: number
  wagon_state: WagonState
  active_situations: Array<{
    situation_id: string
    seat_anchor: Anchor
    type: string // ID сценария, например cold; это не criticality
    pool: 'easy' | 'medium' | 'hard'
  }>
}
type WagonError = { type: 'error'; message: string }
type WagonServerMessage = WagonSnapshot | WagonError

type WagonCommand =
  | { type: 'move_to'; anchor: Anchor }
  | { type: 'pick_item'; item: 'blanket' | 'water' | 'coffee' }
  | { type: 'give_item'; situation_id: string; item: string }
  | { type: 'redirect'; situation_id: string }
```

В `GET /api/situation/{id}` вагонная ситуация дополнительно содержит
`seat_anchor`, `physical_requirement` (`{"kind":"deliver_item","item":"water"}`
или `{"kind":"redirect"}`) и `physical_action_done`. У обычных ситуаций первые
два поля отсутствуют. `physical_action_done` выставляется только сервером.

## WebSocket: подключение и команды

В текущем [client.ts](src/api/client.ts) токены скрыты в переменной модуля;
`getAccessToken()` возвращает **актуальный** access-токен для WS. HTTP
обновляет токен при `401`, WebSocket сам этого не делает. Пример сборки URL:

```ts
const apiOrigin = getApiBaseUrl() || window.location.origin // web; native использует API URL
const url = new URL(wsPath, apiOrigin)
url.protocol = url.protocol === 'https:' ? 'wss:' : 'ws:'
url.searchParams.set('token', accessToken)
const socket = new WebSocket(url.toString())
```

Для React Native вычисляйте `apiOrigin` из `EXPO_PUBLIC_API_URL` /
`getApiBaseUrl()` без обращения к `window`. На Android-эмуляторе базовый
адрес — `http://10.0.2.2:8088`; на физическом устройстве нужен доступный
сетевой адрес сервера. При HTTPS используйте WSS. В production не записывайте
JWT или полный WS URL в логи и аналитику.

После `onopen` сервер сразу присылает `state`. **Заменяйте** локальный снимок
целиком, не объединяйте его по полям. `onmessage` обрабатывает `state` и
`error`; успешная команда подтверждается следующим `state`, отдельного ACK с
`command_id` нет. WebSocket-команды не имеют защиты от повторной отправки:
после разрыва сначала получите новый снимок и не повторяйте `pick_item`
вслепую. У WebSocket-сообщения ошибки пока только текст `message`, без
машинного кода.

| Команда | Когда показывать кнопку |
| --- | --- |
| `move_to` | Целевой якорь есть в карте сцены; игрок сейчас не движется. |
| `pick_item` | `player.at === service_point`, `player.moving` отсутствует; предлагайте предметы из требования открытой ситуации. |
| `give_item` | `physical_requirement.kind === 'deliver_item'`, игрок у `seat.actor.at`, нужный предмет есть в `carried_items`. |
| `redirect` | `physical_requirement.kind === 'redirect'`, игрок у `seat.actor.at`. |

Позиция берётся из `wagon_state.player`, а не из присланного клиентом якоря.
`move_to` длится 3 секунды по времени сервера. Во время `player.moving` все
позиционные команды отклоняются. После `give_item` или `redirect` ситуация
остаётся активной: продолжайте диалог и отдельно вызовите REST `/finish`.
Сервер разрешает взять предмет, указанный хотя бы в одном сценарии каталога,
даже если сейчас никто его не просит; клиенту не следует предлагать
бесцельный подбор предметов.
Если пассажир уже достиг `staff_zone`, его можно вернуть, но
`restricted_reached` и штраф в итоге сохраняются.

## Привязка к 3D и обнаружение

- Создайте таблицу `anchor → Vector3` для всех восьми якорей из
  `wagon_classes.json`: шесть мест, сервисная точка и служебная зона. Нынешние
  четыре `questAnchors` не соответствуют шести местам нового режима.
- Для каждого места рисуйте пассажира по `passenger_def_id` и `actor.at`.
  При `actor.moving` интерполируйте между `from` и `to` за `duration_s` от
  `started_at`. Сверяйте анимацию с каждым серверным снимком; переход
  завершён только когда сервер прислал `moving` без значения и новый `at`.
- Аналогично двигайте проводника по `player.moving`. Нажатие на место
  отправляет `move_to`; локальный 3D-клик сам по себе не меняет игровое
  положение на сервере.
- Сопоставьте `situation_def_id`/`active_situations[].type` с анимацией.
  Появление ситуации обозначайте поведением персонажа, не карточкой задания
  с текстом или автоматическим диалогом. Для неизвестного ID предусмотрите
  нейтральный визуальный сигнал.
- До открытия диалога не показывайте `opening` из REST. Сервер проверяет
  владельца ситуации, но пока **не проверяет близость игрока** для
  `/api/situation/{id}` и `/message`; ограничение взаимодействия по дистанции
  нужно реализовать в UI.

## Переподключение и ошибки

При обрыве WS оставляйте последнюю сцену как временную, показывайте статус
«Переподключение», запрашивайте `GET /api/session/{id}` и открывайте WS снова
с тем же `session_id`. Новый первый `state` полностью заменит старый.
Используйте задержку между попытками и прекращайте их после перехода со
страницы или завершения смены. Если `session.status === 'finished'`, вызовите
идемпотентный `POST /api/session/{id}/finish` для получения разбора.

Если JWT истёк, текущий REST-клиент может обновить его через
`POST /auth/refresh`; после обновления создайте **новое** WS-соединение с
новым access-токеном. Сам `onerror` браузерного WebSocket не сообщает HTTP
код handshake: проверьте авторизацию REST-запросом. `404` при WS подключении
означает чужую/несуществующую смену, `409` — завершённую. `409` при запуске
класса означает `coming_soon` либо отсутствие утверждённого контента в
`official` namespace.

## Проверка готовности фронта

1. Открыть `orientation` в классе `standard`, получить шесть сидящих пассажиров
   и игрока у `service_point`; остальные уровни изначально заблокированы.
2. Получить новую ситуацию через WS без текстового попапа; параллельные
   ситуации не вытесняют друг друга.
3. Открыть диалог у пассажира, увидеть ответ Mock/GigaChat через `/message`.
4. Взять `blanket`, дождаться серверного прибытия к месту и отдать его;
   `physical_action_done` становится `true`, после `/finish` виден скоринг.
5. Разорвать WS и подключиться снова: состояние, места, предметы и активные
   ситуации совпадают с сервером; чужой токен доступа не получает.
6. Завершить смену и показать разбор; новый WS к завершённой смене не
   открывается.
