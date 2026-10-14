# gopanel

Self-hosted web panel for managing a proxy/VPN server.

Один бинарник на Go, вшитый React-фронтенд, SQLite. Панель управляет
ядром (сначала Xray-core, затем — опционально sing-box): генерирует
конфигурацию, добавляет пользователей, считает трафик, отдаёт ссылки
и QR-коды.

**Статус:** ранняя разработка. API и схема БД могут меняться.

## Возможности

- Установка одним бинарником: Go + SQLite, без внешних сервисов.
- Админка слушает **только loopback** (`127.0.0.1`), доступ — через SSH-туннель.
- Вход с паролем (argon2id, ≥12 символов) и TOTP-2FA с recovery-кодами.
- Пользователи с лимитами трафика, сроками и учётом онлайн-IP.
- Инбаунды Xray: VLESS, VMess, Trojan, Shadowsocks, SOCKS, HTTP;
  REALITY, XTLS Vision, XHTTP и другие транспорты.
- Маршрутизация: outbound'ы и правила, пресеты для РФ-сплита.
- Прямые ссылки и QR-коды для подключения.
- Бэкапы: дамп базы, скачивание только с подтверждением пароля.
- Аудит действий администратора.
- Тёмная тема, мобильная вёрстка, `Ctrl-K`.
- i18n: русский и английский.

Пока не реализовано: подписки (публичный listener), лимит устройств,
второе ядро (sing-box), мультинодовость, реселлеры, биллинг.

## Требования

- Linux (Debian/Ubuntu) или Docker.
- Xray-core — устанавливается `install.sh` или Docker-образом.
- Для systemd-установки: root или `sudo`.

## Установка

### Docker (рекомендуется)

```bash
git clone https://github.com/bassvii/gopanel
cd gopanel
docker build --network host -f deploy/Dockerfile -t gopanel:latest .
docker run -d \
    --name gopanel \
    --restart unless-stopped \
    -p 127.0.0.1:44559:44559 \
    -e GOPANEL_LISTEN=0.0.0.0 \
    -e GOPANEL_PORT=44559 \
    -e GOPANEL_ALLOW_NON_LOOPBACK=1 \
    -e GOPANEL_DB_PATH=/var/lib/gopanel/gopanel.db \
    -v gopanel-data:/var/lib/gopanel \
    gopanel:latest
```

Первого администратора создайте вручную:

```bash
docker exec -it gopanel gopanel admin create \
    --db /var/lib/gopanel/gopanel.db \
    --user admin
```

Пароль вводится интерактивно, дважды. После этого панель доступна
по адресу `http://127.0.0.1:44559/<BASE_PATH>/`, где `BASE_PATH`
лежит в файле `/var/lib/gopanel/base_path` внутри контейнера.

Подробнее: [docs/ru/install-docker.md](docs/ru/install-docker.md).

### Systemd (одной командой)

```bash
git clone https://github.com/bassvii/gopanel
cd gopanel
make build
sudo bash deploy/install.sh
```

Установщик скачает Xray и GeoIP/GeoSite, проверит SHA-256,
создаст systemd-unit, пользователя `gopanel`, базу, каталоги
и выведет инструкцию для доступа через SSH-туннель.

Первого администратора создайте по инструкции из вывода:

```bash
sudo -u gopanel /usr/local/bin/gopanel admin create \
    --db /var/lib/gopanel/gopanel.db \
    --user admin
```

Подробнее: [docs/ru/install.md](docs/ru/install.md).

## Безопасность

- Админка слушает `127.0.0.1` — снаружи недоступна. Доступ через
  SSH-туннель: `ssh -L <port>:127.0.0.1:<port> user@server`.
- Пароли хранятся argon2id, TOTP-секреты шифруются AES-256-GCM
  ключом вне базы. Токены подписок — только в виде хешей.
- CSRF-защита на всех изменяющих запросах.
- Rate limit по IP и по аккаунту; lockout снимается валидным TOTP.
- Все действия администратора пишутся в аудит-лог.

Полная модель угроз и как сообщить об уязвимости: [SECURITY.md](SECURITY.md).

## Документация

- [docs/ru/install.md](docs/ru/install.md) — установка через systemd.
- [docs/ru/install-docker.md](docs/ru/install-docker.md) — установка в Docker.
- [docs/ru/clients.md](docs/ru/clients.md) — подключение клиентов.
- [docs/ru/xray-notes.md](docs/ru/xray-notes.md) — заметки по Xray API.
- [CONTRIBUTING.md](CONTRIBUTING.md) — правила вклада.
- [SECURITY.md](SECURITY.md) — модель угроз и отчёты об уязвимостях.

## Разработка

```bash
git clone https://github.com/bassvii/gopanel
cd gopanel

# фронтенд
cd web && npm ci && cd ..

# собрать всё
make build                 # или: go build -o bin/gopanel ./cmd/gopanel

# запустить локально
./bin/gopanel admin create --db /tmp/gp.db --user admin
./bin/gopanel run --db /tmp/gp.db
```

Тесты:

```bash
go test ./...
cd web && npm run test:e2e
```

Правила вклада: [CONTRIBUTING.md](CONTRIBUTING.md) (DCO, `git commit -s`).

## Лицензия

AGPL-3.0-or-later. См. [LICENSE](LICENSE).
