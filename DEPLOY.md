# Деплой

Бот работает на long polling, поэтому **никаких портов не публикует**: ни
домена, ни сертификата, ни reverse proxy. Наружу он делает только исходящие
запросы к `api.telegram.org`, `api.open-meteo.com` и
`geocoding-api.open-meteo.com`. На сервере, где уже живут другие приложения
за Caddy, конфликтовать нечему: `docker-compose.yml` не содержит секции
`ports` и не подключается к сети `web`.

Единственные имена, которые должны быть уникальными на машине: контейнер
`weatherbot` и docker-том `weatherfit_weatherfit-data` (имя тома Compose
складывает из имени каталога проекта).

## 1. Токен

1. Напиши [@BotFather](https://t.me/BotFather), команда `/newbot`.
2. Скопируй токен вида `123456789:AA...`.
3. Там же, при желании: `/setdescription`, `/setuserpic`. Команды бот
   регистрирует сам при старте.

## 2. Подготовка сервера

Если на этой машине уже развёрнуты другие проекты, шаг можно пропустить:
пользователь `deploy`, Docker и ufw уже настроены. Для чистого сервера:

```bash
ssh root@SERVER_IP

adduser deploy
usermod -aG sudo deploy

apt update && apt install -y ufw unattended-upgrades
ufw allow OpenSSH
ufw enable                      # 80 и 443 этому боту не нужны

curl -fsSL https://get.docker.com | sh
usermod -aG docker deploy
```

Дальше всё от имени `deploy`, не `root`.

## 3. Деплой-ключ: сервер читает GitHub

Репозиторий приватный, поэтому серверу нужен свой доступ на чтение. Если он
уже настроен для другого проекта на этой машине, ключ можно переиспользовать:
добавь тот же публичный ключ в deploy keys этого репозитория.

```bash
# на сервере, от deploy
ssh-keygen -t ed25519 -f ~/.ssh/id_ed25519 -C "weatherfit-server" -N ""
cat ~/.ssh/id_ed25519.pub
```

Публичный ключ вставить в GitHub: репозиторий → Settings → Deploy keys → Add
deploy key. Галку «Allow write access» **не ставить**, сервер только читает.

Не путай этот ключ с `DEPLOY_SSH_KEY` из раздела про секреты: здесь сервер
читает GitHub, там GitHub Actions заходит на сервер. Это разные ключи и
разные направления.

## 4. Клонирование и конфигурация

```bash
git clone git@github.com:sdimitrenco/weatherfit.git
cd weatherfit
cp .env.example .env
nano .env
```

Обязательна одна переменная: `TELEGRAM_BOT_TOKEN`. Остальные задают
настройки нового подписчика, каждый меняет их себе сам через `/settings`.

| Переменная | Значение |
|---|---|
| `TELEGRAM_BOT_TOKEN` | токен от @BotFather |
| `TELEGRAM_ALLOWED_CHAT_IDS` | пусто, если бот открыт для всех; свой chat_id, если только для себя |
| `TELEGRAM_ADMIN_CHAT_IDS` | свой chat_id, чтобы работала `/stats` |
| `DEFAULT_LANG` | `en`, `ru` или `de` для тех, у кого Telegram не прислал язык |
| `DB_PATH` | не менять: в контейнере переопределяется на `/data/weatherfit.db` |

Свой chat_id узнать просто: оставь `TELEGRAM_ALLOWED_CHAT_IDS` пустым,
напиши боту любое сообщение и найди в логе строку
`сообщение от неизвестного chat_id`.

## 5. Первый запуск

```bash
docker compose up -d --build
docker compose logs -f
```

В логе должна появиться строка `бот запущен`. Дальше напиши боту `/start`.

## 6. Секреты GitHub Actions

Settings → Secrets and variables → Actions → New repository secret:

| Секрет | Значение |
|---|---|
| `DEPLOY_HOST` | IP сервера |
| `DEPLOY_USER` | `deploy` |
| `DEPLOY_SSH_KEY` | **приватная** половина ключа, который принимает сервер |

Ключ лучше отдельный, а не личный. Если для другого проекта на этом сервере
он уже создан, тот же приватный ключ можно вставить и здесь:

```bash
# на своей машине
ssh-keygen -t ed25519 -f ~/.ssh/weatherfit-deploy -C "github-actions" -N ""
ssh-copy-id -i ~/.ssh/weatherfit-deploy.pub deploy@SERVER_IP
cat ~/.ssh/weatherfit-deploy        # это целиком в DEPLOY_SSH_KEY
```

Проверить до того, как полагаться на воркфлоу:

```bash
ssh -i ~/.ssh/weatherfit-deploy deploy@SERVER_IP "cd ~/weatherfit && git status"
```

## 7. Что делает CI и Deploy

- **CI** (`.github/workflows/ci.yml`) на каждый push в `main` и на каждый
  pull request: `gofmt`, `go build`, `go vet`, тесты с детектором гонок,
  `golangci-lint`, сборка docker-образа и проверка, что контейнер падает с
  кодом 1 без токена.
- **Deploy** (`.github/workflows/deploy.yml`) ждёт успешного CI, заходит на
  сервер по SSH, делает `git reset --hard origin/main`, пересобирает и
  перезапускает контейнер, чистит старые образы. Затем проверяет, что
  контейнер в состоянии `running` и в логе есть `бот запущен`, иначе падает.

Deploy срабатывает только для коммитов, начинающихся с `feat:` или `fix:`,
на ветке `main`. Коммит `chore:` или `docs:` сервер не трогает. Есть кнопка
**Run workflow** для деплоя без нового коммита.

## 8. Обновление руками

```bash
cd ~/weatherfit
git pull
docker compose up -d --build
```

База подписчиков лежит в docker-томе и обновление её не трогает.

## 9. Перезапуск сервера

`restart: unless-stopped` поднимает контейнер после перезагрузки. Если
сервис лежал в момент утренней рассылки, при старте бот досылает
пропущенный отчёт всем, кому он ещё не ушёл, но не позже конца активного
окна подписчика.

## 10. Бэкап

Всё состояние это один файл SQLite:

```bash
docker compose stop
docker run --rm -v weatherfit_weatherfit-data:/data -v "$PWD":/backup busybox \
  cp /data/weatherfit.db /backup/weatherfit-$(date +%F).db
docker compose start
```

Восстановление обратное: положить файл в том и поднять контейнер. Точное имя
тома на своей машине: `docker volume ls | grep weatherfit`.

## 11. Проверка утренней рассылки

Не дожидаясь утра: в боте `/settings` → «Сменить время», пришли время на
минуту вперёд. Сообщение придёт в начале следующей минуты. Потом верни
обычное время.

## 12. Диагностика

| Симптом | Что смотреть |
|---|---|
| Бот молчит | `docker compose logs --tail=50`, `unauthorized` означает неверный токен |
| «Это приватный бот» | твой chat_id не в `TELEGRAM_ALLOWED_CHAT_IDS` |
| Нет утренней рассылки | в логе `утренние отчёты отправлены`; проверь в `/settings`, не на паузе ли |
| «Не удалось получить прогноз» | Open-Meteo недоступен; бот сам ретраит 5 раз за ~15 минут |
| Контейнер перезапускается | `docker inspect -f '{{.RestartCount}}' weatherbot`, потом логи: обычно битый токен или недоступный том |
| Deploy пропущен | коммит не начинается с `feat:`/`fix:`, либо CI упал; есть кнопка **Run workflow** |

## Без Docker

```bash
make build
DB_PATH=/var/lib/weatherbot/weatherfit.db ./bin/weatherbot
```

Для systemd подойдёт юнит с `Restart=always`,
`EnvironmentFile=/opt/weatherbot/.env` и `User=weatherbot`. Каталог под
`DB_PATH` должен быть доступен этому пользователю на запись.
