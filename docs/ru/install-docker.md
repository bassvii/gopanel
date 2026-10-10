# Установка gopanel в Docker

## Требования

- Linux-сервер (или WSL 2)
- Docker 24+
- Docker Compose v2 (встроен в `docker` CLI)
- root-доступ или пользователь в группе `docker`

## Содержание

- [Быстрый старт](#быстрый-старт)
- [Сборка образа](#сборка-образа)
- [Запуск контейнера](#запуск-контейнера)
- [Docker Compose](#docker-compose)
- [Порты инбаундов](#порты-инбаундов)
- [Управление](#управление)
- [Обновление](#обновление)
- [Резервные копии](#резервные-копии)
- [Удаление](#удаление)
- [Диагностика](#диагностика)

---

## Быстрый старт

Минимальный набор команд:

```bash
# 1. Клонировать репозиторий
git clone https://github.com/bassvii/gopanel
cd gopanel

# 2. Собрать образ
docker build --network host -f deploy/Dockerfile -t gopanel:latest .

# 3. Запустить
docker run -d \
    --name gopanel \
    --restart unless-stopped \
    -p 127.0.0.1:44559:44559 \
    -e GOPANEL_LISTEN=0.0.0.0 \
    -e GOPANEL_PORT=44559 \
    -e GOPANEL_ALLOW_NON_LOOPBACK=1 \
    -e GOPANEL_DB_PATH=/var/lib/gopanel/gopanel.db \
    -e GOPANEL_ADMIN_USER=admin \
    -e GOPANEL_ADMIN_PASSWORD='сложный-пароль-минимум-12-символов' \
    -v gopanel-data:/var/lib/gopanel \
    gopanel:latest

# 4. Проверить логи
docker logs gopanel

# 5. Открыть панель
# http://127.0.0.1:44559/BASE_PATH/
# BASE_PATH — из логов, строка base_path=...
```

---

## Сборка образа

### Из репозитория

```bash
git clone https://github.com/bassvii/gopanel
cd gopanel
docker build --network host -f deploy/Dockerfile -t gopanel:latest .
```

Флаг `--network host` нужен, потому что при сборке `npm ci` внутри Docker может не иметь доступа к реестру из-за IPv6. С `--network host` контейнер использует сеть хоста.

### Проверка

```bash
docker images | grep gopanel
# → gopanel  latest  abc123def456  10 seconds ago  50MB
```

### Если сборка падает на `npm ci` с ETIMEDOUT

**Причина:** Docker-контейнер не может достучаться до `registry.npmjs.org` из-за IPv6.

**Решение 1** — `--network host` (уже в команде выше).

**Решение 2** — добавить `NODE_OPTIONS` в Dockerfile. Откройте `deploy/Dockerfile`, стадия `web`:

```dockerfile
FROM node:24-alpine AS web
WORKDIR /web
ENV NODE_OPTIONS=--dns-result-order=ipv4first
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build
```

**Решение 3** — зеркало npm. В `web/.npmrc`:

```text
registry=https://registry.npmmirror.com
```

И в Dockerfile скопировать `.npmrc`:

```dockerfile
COPY web/package.json web/package-lock.json web/.npmrc ./
```

### Если сборка падает на `go build`

Проверьте локально:

```bash
cd ~/projects/gopanel
go build -o bin/gopanel ./cmd/gopanel
```

Если локально собирается, а в Docker — нет, покажите ошибку. Чаще всего — забытый импорт или неиспользуемая переменная.

---

## Запуск контейнера

### Минимальный запуск

```bash
docker run -d \
    --name gopanel \
    --restart unless-stopped \
    -p 127.0.0.1:44559:44559 \
    -e GOPANEL_LISTEN=0.0.0.0 \
    -e GOPANEL_PORT=44559 \
    -e GOPANEL_ALLOW_NON_LOOPBACK=1 \
    -e GOPANEL_DB_PATH=/var/lib/gopanel/gopanel.db \
    -e GOPANEL_ADMIN_USER=admin \
    -e GOPANEL_ADMIN_PASSWORD='сложный-пароль' \
    -v gopanel-data:/var/lib/gopanel \
    gopanel:latest
```

### Флаги

| Флаг | Что делает |
|------|------------|
| `-d` | фоновый режим |
| `--name gopanel` | имя контейнера |
| `--restart unless-stopped` | автозапуск при перезагрузке |
| `-p 127.0.0.1:44559:44559` | проброс порта только на localhost хоста |
| `-e GOPANEL_LISTEN=0.0.0.0` | панель слушает все интерфейсы внутри контейнера |
| `-e GOPANEL_PORT=44559` | фиксированный порт (иначе случайный) |
| `-e GOPANEL_ALLOW_NON_LOOPBACK=1` | разрешить не-loopback (обязательно для Docker) |
| `-e GOPANEL_DB_PATH=/var/lib/gopanel/gopanel.db` | путь к БД внутри контейнера |
| `-e GOPANEL_ADMIN_USER=admin` | имя первого админа (создаётся один раз) |
| `-e GOPANEL_ADMIN_PASSWORD=...` | пароль первого админа |
| `-v gopanel-data:/var/lib/gopanel` | именованный том для БД |

### Важно про порты

- Слева — адрес на хосте. `127.0.0.1:44559` означает: «слушать только на localhost хоста».
- Справа — порт внутри контейнера. Должен совпадать с `GOPANEL_PORT`.

**Безопасность.** Панель доступна только с самого сервера. Снаружи — через SSH-туннель:

```bash
ssh -L 44559:127.0.0.1:44559 user@your-server
```

Затем в браузере: `http://127.0.0.1:44559/BASE_PATH/`.

### Проверка

```bash
docker logs gopanel
```

Ожидаемо:

```text
level=INFO msg="first admin created from env" username=admin id=1
level=INFO msg="xray started" pid=22 config=/tmp/gopanel-xray.json
level=INFO msg="frontend embedded"
level=INFO msg="admin listener started" addr=[::]:44559 base_path=/7fee39ed0cc3e93666be46446375cffc
```

Запишите `base_path` — это `BASE_PATH` для URL.

Проверьте, что панель отвечает:

```bash
BASE=/7fee39ed0cc3e93666be46446375cffc

curl -s -o /dev/null -w "%{http_code}\n" "http://127.0.0.1:44559$BASE/me"
# → 401
```

`401` — правильно, панель работает и требует авторизации.

### Вход

1. Откройте `http://127.0.0.1:44559/BASE_PATH/`.
2. Логин: `admin`.
3. Пароль: из `GOPANEL_ADMIN_PASSWORD`.

---

## Docker Compose

Удобнее для постоянного использования.

### 1. Создайте `docker-compose.yml`

```bash
mkdir -p /opt/gopanel
cd /opt/gopanel
nano docker-compose.yml
```

```yaml
services:
  gopanel:
    build:
      context: https://github.com/bassvii/gopanel.git
      dockerfile: deploy/Dockerfile
    image: gopanel:latest
    container_name: gopanel
    restart: unless-stopped
    ports:
      - "127.0.0.1:44559:44559"
      # Для инбаундов раскомментируйте и укажите свои порты:
      # - "443:443"
      # - "8443:8443"
    environment:
      GOPANEL_LISTEN: "0.0.0.0"
      GOPANEL_PORT: "44559"
      GOPANEL_ALLOW_NON_LOOPBACK: "1"
      GOPANEL_DB_PATH: "/var/lib/gopanel/gopanel.db"
      GOPANEL_ADMIN_USER: "admin"
      GOPANEL_ADMIN_PASSWORD: "сложный-пароль-минимум-12-символов"
    volumes:
      - gopanel-data:/var/lib/gopanel

volumes:
  gopanel-data:
```

### 2. Запуск

```bash
cd /opt/gopanel
docker compose build
docker compose up -d
```

Первый `build` скачает репозиторий и соберёт образ.

### 3. Проверка

```bash
docker compose ps
# → gopanel  running

docker compose logs -f gopanel
```

### 4. Управление

```bash
docker compose stop            # остановить
docker compose start           # запустить
docker compose restart         # перезапустить
docker compose logs -f         # логи в реальном времени
docker compose ps              # статус
docker compose exec gopanel sh # войти внутрь
```

### 5. Обновление

```bash
docker compose build --no-cache
docker compose up -d --force-recreate
```

Данные в томе `gopanel-data` сохранятся.

---

## Порты инбаундов

Если вы создаёте инбаунды в панели — их порты должны быть проброшены из контейнера на хост.

Пример: VLESS на порту 443, Shadowsocks на 8388.

### Через `docker run`

```bash
docker run -d \
    --name gopanel \
    -p 127.0.0.1:44559:44559 \
    -p 443:443 \
    -p 8388:8388 \
    ... \
    gopanel:latest
```

### Через `docker-compose.yml`

```yaml
ports:
  - "127.0.0.1:44559:44559"
  - "443:443"
  - "8388:8388"
```

**Важно:** панель должна создавать инбаунды на тех же портах, что проброшены. Если в панели инбаунд на 443, но в Docker проброшен только 44559 — клиенты не подключатся.

### Порт 443 и права

Xray внутри контейнера запускается от непривилегированного пользователя `gopanel`. Порт 443 (< 1024) требует прав.

В Dockerfile должен быть `setcap`:

```dockerfile
RUN apk add --no-cache libcap && \
    setcap 'cap_net_bind_service=+ep' /usr/local/bin/xray
```

Если `setcap` нет — используйте порт > 1024 (например, 8443).

---

## Управление

### Логи

```bash
docker logs gopanel
docker logs -f gopanel           # в реальном времени
docker logs --tail 100 gopanel   # последние 100 строк
```

### Статус

```bash
docker ps | grep gopanel
docker inspect gopanel | grep -i status
```

### Перезапуск

```bash
docker restart gopanel
```

### Войти внутрь

```bash
docker exec -it gopanel sh
```

Полезно для:

- Просмотра `/tmp/gopanel-xray.json`
- Запуска CLI-команд (`gopanel admin create`, `gopanel backup`)
- Проверки Xray (`xray version`, `xray api statsquery`)

### Сменить пароль админа

```bash
docker exec gopanel /usr/local/bin/gopanel reset-password \
    --db /var/lib/gopanel/gopanel.db \
    --user admin --password 'новый-пароль'
```

Сбрасывает пароль, отключает 2FA, завершает все сессии.

---

## Обновление

### Через `docker run`

```bash
# 1. Остановить контейнер
docker stop gopanel
docker rm gopanel

# 2. Пересобрать образ
cd ~/projects/gopanel
git pull
docker build --network host -f deploy/Dockerfile -t gopanel:latest .

# 3. Запустить заново (с теми же флагами)
docker run -d \
    --name gopanel \
    ... \
    -v gopanel-data:/var/lib/gopanel \
    gopanel:latest
```

Том `gopanel-data` сохраняется — данные не теряются.

### Через Docker Compose

```bash
cd /opt/gopanel
docker compose build --no-cache
docker compose up -d --force-recreate
```

### Перед обновлением — бэкап

```bash
docker exec gopanel /usr/local/bin/gopanel backup \
    --db /var/lib/gopanel/gopanel.db \
    --out /var/lib/gopanel/backup-before-update.db

docker cp gopanel:/var/lib/gopanel/backup-before-update.db ./backup.db
```

---

## Резервные копии

### Создать бэкап

```bash
# Внутри контейнера
docker exec gopanel /usr/local/bin/gopanel backup \
    --db /var/lib/gopanel/gopanel.db \
    --out /var/lib/gopanel/backup.db

# Скопировать на хост
docker cp gopanel:/var/lib/gopanel/backup.db ./backup-$(date +%Y%m%d).db

# Проверить
sqlite3 ./backup-$(date +%Y%m%d).db ".tables"
```

### Через UI

**Настройки → Резервные копии → Скачать бэкап** — скачивает дамп через браузер.

### Восстановление

```bash
# 1. Остановить контейнер
docker stop gopanel

# 2. Скопировать бэкап в контейнер
docker cp ./backup-20261010.db gopanel:/var/lib/gopanel/gopanel.db

# 3. Запустить
docker start gopanel

# 4. Проверить логи
docker logs gopanel
```

**Проблема:** `docker cp` работает только для существующего контейнера. Если контейнер удалён — восстанавливайте через том:

```bash
# Создать временный контейнер с тем же томом
docker run --rm -it \
    -v gopanel-data:/data \
    -v $(pwd):/backup \
    alpine sh

# Внутри контейнера:
cp /backup/backup-20261010.db /data/gopanel.db
chown 999:999 /data/gopanel.db
exit
```

`999:999` — UID/GID пользователя `gopanel` внутри контейнера.

### Автоматический бэкап (cron на хосте)

```bash
sudo crontab -e
```

```cron
0 3 * * * docker exec gopanel /usr/local/bin/gopanel backup --db /var/lib/gopanel/gopanel.db --out /var/lib/gopanel/backup-auto.db && docker cp gopanel:/var/lib/gopanel/backup-auto.db /backups/gopanel-$(date +\%Y\%m\%d).db
30 3 * * * find /backups -name "gopanel-*.db" -mtime +30 -delete
```

---

## Удаление

### Остановить и удалить контейнер

```bash
docker stop gopanel
docker rm gopanel
```

### Удалить образ

```bash
docker rmi gopanel:latest
```

### Удалить данные

**ОСТОРОЖНО:** удалит всех пользователей, инбаунды, настройки.

```bash
docker volume rm gopanel-data
```

### Через Docker Compose

```bash
cd /opt/gopanel
docker compose down       # контейнер, данные сохраняются
docker compose down -v    # контейнер + том (ОСТОРОЖНО)
docker rmi gopanel:latest
```

---

## Диагностика

### Контейнер не запускается

```bash
docker logs gopanel
```

| Ошибка | Причина | Решение |
|--------|---------|---------|
| `xray binary not found` | Образ собран без Xray | Пересоберите образ из свежего Dockerfile |
| `bind: address already in use` | Порт занят | Смените `-p` или убейте процесс |
| `database is locked` | Том смонтирован неверно | Проверьте `-v gopanel-data:/var/lib/gopanel` |
| `permission denied` на `/var/lib/gopanel` | Права тома | `docker run ... --user $(id -u):$(id -g)` или `chown` внутри |

### Панель не отвечает

```bash
# Порт слушается на хосте?
ss -tlnp | grep 44559
# → должен быть docker-proxy

# Внутри контейнера?
docker exec gopanel ss -tlnp | grep 44559
```

### Не открывается в браузере

Проверьте SSH-туннель:

```bash
ssh -L 44559:127.0.0.1:44559 user@server
```

Проверьте `BASE_PATH`:

```bash
docker logs gopanel 2>&1 | grep "admin listener started"
```

### `npm ci` при сборке падает с ETIMEDOUT

```bash
docker build --network host -f deploy/Dockerfile -t gopanel:latest .
```

Или добавьте в Dockerfile:

```dockerfile
ENV NODE_OPTIONS=--dns-result-order=ipv4first
```

### Контейнер работает, но инбаунды недоступны снаружи

**Причина:** порты инбаундов не проброшены.

**Решение:** добавьте `-p <port>:<port>` в `docker run` или в `docker-compose.yml`.

### Xray не может занять порт 443

Внутри контейнера Xray работает от непривилегированного пользователя.

**Решение 1:** используйте порт > 1024 (например, 8443).

**Решение 2:** добавьте в Dockerfile:

```dockerfile
RUN apk add --no-cache libcap && \
    setcap 'cap_net_bind_service=+ep' /usr/local/bin/xray
```

### Панель перезапускается циклически

```bash
docker logs gopanel
docker inspect gopanel | grep -i "restart"
```

Частая причина: Xray падает на конфиге. Проверьте:

```bash
docker exec gopanel cat /tmp/gopanel-xray.json | head -50
docker exec gopanel xray run -test -c /tmp/gopanel-xray.json
```
```
