# Деплой

Бот работает на long polling, поэтому входящие порты не нужны: ни домена, ни
сертификата, ни проброса портов. Наружу он делает только исходящие запросы к
`api.telegram.org`, `api.open-meteo.com` и `geocoding-api.open-meteo.com`.

## 1. Токен

1. Напиши [@BotFather](https://t.me/BotFather), команда `/newbot`.
2. Скопируй токен вида `123456789:AA...`.
3. Там же, при желании: `/setdescription`, `/setuserpic`. Команды бот
   регистрирует сам при старте.

## 2. Файлы на сервере

```bash
git clone <репозиторий> /opt/weatherbot
cd /opt/weatherbot
cp .env.example .env
$EDITOR .env          # как минимум TELEGRAM_BOT_TOKEN
```

Если бот только для тебя, впиши свой chat_id в `TELEGRAM_ALLOWED_CHAT_IDS`.
Свой chat_id узнать просто: оставь список пустым, напиши боту любое сообщение
и найди в логе строку `сообщение от неизвестного chat_id`. Ту же цифру
пропиши в `TELEGRAM_ADMIN_CHAT_IDS`, чтобы работала команда `/stats`.

## 3. Запуск

```bash
docker compose up -d --build
docker compose logs -f
```

В логе должна появиться строка `бот запущен`. Дальше напиши боту `/start`.

## 4. Обновление

```bash
cd /opt/weatherbot
git pull
docker compose up -d --build
```

База подписчиков лежит в docker-томе `weatherfit-data` и обновление её не
трогает.

## 5. Перезапуск сервера

`restart: unless-stopped` поднимает контейнер после перезагрузки. Если
сервис лежал в момент утренней рассылки, при старте бот досылает
пропущенный отчёт всем, кому он ещё не ушёл, но не позже конца активного
окна подписчика.

## 6. Бэкап

Вся состояние это один файл SQLite:

```bash
docker compose stop
docker run --rm -v weatherfit-data:/data -v "$PWD":/backup busybox \
  cp /data/weatherfit.db /backup/weatherfit-$(date +%F).db
docker compose start
```

Восстановление обратное: положить файл в том и поднять контейнер.

## 7. Проверка утренней рассылки

Не дожидаясь утра: в боте `/settings` → «Сменить время», пришли время на
минуту вперёд. Сообщение придёт в начале следующей минуты. Потом верни
обычное время.

## 8. Диагностика

| Симптом | Что смотреть |
|---|---|
| Бот молчит | `docker compose logs --tail=50`, `unauthorized` означает неверный токен |
| «Это приватный бот» | твой chat_id не в `TELEGRAM_ALLOWED_CHAT_IDS` |
| Нет утренней рассылки | в логе `утренние отчёты отправлены`; проверь `/settings`, не на паузе ли |
| «Не удалось получить прогноз» | Open-Meteo недоступен; бот сам ретраит 5 раз за ~15 минут |

## Без Docker

```bash
make build
DB_PATH=/var/lib/weatherbot/weatherfit.db ./bin/weatherbot
```

Для systemd подойдёт юнит с `Restart=always`, `EnvironmentFile=/opt/weatherbot/.env`
и `User=weatherbot`. Каталог под `DB_PATH` должен быть доступен этому пользователю на запись.
