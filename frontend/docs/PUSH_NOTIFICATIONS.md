# Push-уведомления Android

Приложение получает Expo push token после разрешения пользователя и отправляет его в `POST /api/me/push-subscriptions`. Бэкенд посылает уведомления через Expo Push Service, когда открывается новый урок или до истечения призовых баллов остаётся менее 24 часов. Нажатие на уведомление ведёт к маршруту уроков или профилю с призовым балансом.

## Что нужно для реальной доставки

1. Создать Expo/EAS проект и взять его `projectId`. Перед сборкой задать `EXPO_PUBLIC_EAS_PROJECT_ID` в окружении фронтенда. Его также можно хранить в `expo.extra.eas.projectId`.
2. Создать Android-приложение `ru.moscowtransport.vsmtrainer` в Firebase. Скачать `google-services.json`, указать путь к нему в `expo.android.googleServicesFile` и пересобрать нативный проект. Для существующей папки `android/` одного изменения `app.json` недостаточно: после настройки выполнить `npx expo prebuild --platform android` и проверить изменения в нативном проекте.
3. В EAS загрузить секретный FCM V1 service account key через `eas credentials`. Этот ключ не добавлять в Git. Если в Expo включена дополнительная защита push API, задать `EXPO_PUSH_ACCESS_TOKEN` для контейнера бэкенда.
4. Собрать и установить собственный Android APK/development build. Remote push в Expo Go не работает. При сборке для физического телефона задать `EXPO_PUBLIC_API_URL` с доступным телефону адресом бэкенда, например `http://192.168.1.10:8088`.
5. В приложении войти в профиль и нажать «Включить уведомления». После согласия устройство регистрируется на бэкенде. При следующем входе разрешённый токен регистрируется снова для текущего аккаунта.

Бэкенд в Docker по умолчанию использует `PUSH_MODE=expo`. Для локальной отладки без доставки можно задать `PUSH_MODE=log`: сообщения будут записываться в лог контейнера.

Проверка доставки: после регистрации устройства завершить B01 так, чтобы открылся B02, либо использовать [Expo push notifications tool](https://expo.dev/notifications) с токеном устройства. Для проверки отправки с бэкенда смотрите `docker compose logs backend`: ошибки Expo Push Service выводятся в лог. Успешный push ticket означает принятие сообщения сервисом Expo, а не подтверждение получения телефоном.

См. [настройку Expo](https://docs.expo.dev/push-notifications/push-notifications-setup/), [FCM V1](https://docs.expo.dev/push-notifications/fcm-credentials/) и [отправку push](https://docs.expo.dev/push-notifications/sending-notifications/).
