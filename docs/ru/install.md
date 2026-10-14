# Установка gopanel

## Требования

- Linux (Debian 12+, Ubuntu 22.04+)
- root-доступ (или sudo)
- x86_64 или arm64
- **Go 1.27.2+** и **Node.js 20+** — только если собираете из исходников
- ~100 МБ свободного места

## Установка

### 1. Клонировать и собрать

```bash
git clone https://github.com/bassvii/gopanel
cd gopanel
make build
```

Если `make` недоступен:

```bash
cd web && npm ci && npm run build && cd ..
go build -o bin/gopanel ./cmd/gopanel
```

### 2. Запустить установщик

```bash
sudo bash deploy/install.sh
```

Скрипт:

1. Установит зависимости (`curl`, `unzip`, `sqlite3`, `ca-certificates`).
2. Скачает **Xray-core v26.3.27** из официального релиза и проверит SHA-256.
3. Скачает **GeoIP/GeoSite** (Loyalsoldier, тег `202610132205`) с проверкой SHA-256.
4. Создаст системного пользователя `gopanel` (без shell, без home).
5. Установит бинарник в `/usr/local/bin/gopanel`.
6. Создаст каталоги `/var/lib/gopanel` (0700) и `/etc/gopanel`.
7. Инициализирует базу `/var/lib/gopanel/gopanel.db` (0600).
8. Установит и запустит systemd-сервис `gopanel`.
9. Напечатает URL панели и инструкцию для SSH-туннеля.

## Первый админ

После установки создайте администратора **вручную** (скрипт этого не делает):

```bash
sudo -u gopanel /usr/local/bin/gopanel admin create \
    --db /var/lib/gopanel/gopanel.db \
    --user admin
```

Пароль вводится **интерактивно** (дважды, без эха). Минимум 12 символов.

Флаг `--password` **удалён** в v0.1: он попадал в `ps` и shell history. Для CI/автоматизации есть `GOPANEL_ADMIN_PASSWORD` env, но использовать его в проде не рекомендуется.

## Доступ к панели

Панель слушает **только `127.0.0.1`**. Доступ — через SSH-туннель:

```bash
ssh -L <PORT>:127.0.0.1:<PORT> user@server
```

`PORT` и `BASE_PATH` выводятся в конце `install.sh` и доступны так:

```bash
# Порт из БД
sqlite3 /var/lib/gopanel/gopanel.db "SELECT value FROM settings WHERE key='admin_port'"

# Секретный путь из файла
sudo cat /var/lib/gopanel/base_path
```

Затем в браузере:

```
http://127.0.0.1:<PORT><BASE_PATH>/
```

## Обновление

Заменить бинарник, не трогая базу:

```bash
cd gopanel
git pull
make build
sudo systemctl stop gopanel
sudo install -m 0755 bin/gopanel /usr/local/bin/gopanel
sudo systemctl start gopanel
```

База `/var/lib/gopanel/gopanel.db` сохраняется. Миграции применяются автоматически при старте.

## Управление

```bash
systemctl status gopanel
systemctl restart gopanel
systemctl stop gopanel
systemctl start gopanel

# Логи
journalctl -u gopanel -f
journalctl -u gopanel -n 50
```

### Проверка

```bash
# Панель жива?
curl -s -o /dev/null -w "%{http_code}\n" "http://127.0.0.1:$(sqlite3 /var/lib/gopanel/gopanel.db "SELECT value FROM settings WHERE key='admin_port'")/healthz"
# → 200
```

## Резервные копии

### Создать бэкап

```bash
sudo -u gopanel /usr/local/bin/gopanel backup \
    --db /var/lib/gopanel/gopanel.db \
    --out /var/lib/gopanel/backup-$(date +%Y%m%d).db
```

### Восстановление

```bash
sudo systemctl stop gopanel

# Сделать снимок текущей базы (на случай отката)
sudo cp /var/lib/gopanel/gopanel.db /var/lib/gopanel/gopanel.db.before-restore

# Заменить базу
sudo cp backup-20261014.db /var/lib/gopanel/gopanel.db
sudo chown gopanel:gopanel /var/lib/gopanel/gopanel.db
sudo chmod 0600 /var/lib/gopanel/gopanel.db

sudo systemctl start gopanel
```

### Шифрование бэкапа

Если используете `scrypt`/`gpg`/`age` — шифруйте **файл бэкапа**, не базу. Ключ храните **вне** сервера.

## Удаление

### Полное удаление

```bash
sudo systemctl stop gopanel
sudo systemctl disable gopanel
sudo rm /etc/systemd/system/gopanel.service
sudo rm /usr/local/bin/gopanel
sudo rm -rf /var/lib/gopanel /etc/gopanel
sudo userdel gopanel

# Xray (если не нужен другим)
sudo rm /usr/local/bin/xray
sudo rm -rf /usr/local/share/xray
```

**Осторожно:** `rm -rf /var/lib/gopanel` удалит базу со всеми пользователями, инбаундами и настройками. Сделайте бэкап заранее, если он нужен.

### Только остановить

```bash
sudo systemctl stop gopanel
sudo systemctl disable gopanel
```

Данные останутся в `/var/lib/gopanel`.

## Firewall

**Hysteria2 использует UDP.** Если на сервере включён firewall, откройте UDP-порт инбаунда.

### ufw

```bash
sudo ufw allow 443/udp
sudo ufw allow 8443/udp
sudo ufw reload
```

### firewalld

```bash
sudo firewall-cmd --permanent --add-port=443/udp
sudo firewall-cmd --reload
```

### iptables

```bash
sudo iptables -A INPUT -p udp --dport 443 -j ACCEPT
sudo iptables -A INPUT -p udp --dport 8443 -j ACCEPT
sudo netfilter-persistent save
```

### Docker

В `docker-compose.yml` UDP-порты указывайте **с суффиксом `/udp`**:

```yaml
ports:
  - "443:443/udp"
  - "8443:8443/udp"
```

Без `/udp` Docker пробросит только TCP, и Hysteria2 не будет работать.

## Диагностика

### Панель не запускается

```bash
sudo systemctl status gopanel
sudo journalctl -u gopanel -n 50
```

Частые причины:

- **«в базе нет администратора»** — нормально при первом запуске, создайте админа (см. выше).
- **«address already in use»** — порт занят. Смените `--port` или остановите конфликтующий процесс.
- **«permission denied»** на порт < 1024 — Xray не может занять порт. Проверьте:
  ```bash
  systemctl show gopanel | grep -E 'AmbientCap|NoNewPriv'
  ```
  Unit должен содержать `AmbientCapabilities=CAP_NET_BIND_SERVICE` и `NoNewPrivileges=false`. См. `deploy/install.sh`.

### Не открывается в браузере

1. Проверьте SSH-туннель: `ssh -L <PORT>:127.0.0.1:<PORT> user@server`.
2. Проверьте `BASE_PATH`: `sudo cat /var/lib/gopanel/base_path`.
3. Проверьте, что панель слушает `127.0.0.1`:
   ```bash
   ss -tlnp | grep gopanel
   ```

### Порт 443 не биндится

Если Xray не может занять порт 443, проверьте unit:

```bash
systemctl cat gopanel | grep -E 'Ambient|Capability|NoNewPriv'
```

Должно быть:

```ini
NoNewPrivileges=false
AmbientCapabilities=CAP_NET_BIND_SERVICE
CapabilityBoundingSet=CAP_NET_BIND_SERVICE
```

## См. также

- [install-docker.md](install-docker.md) — установка в Docker.
- [xray-notes.md](xray-notes.md) — заметки по Xray API и hot-reload.
- [clients.md](clients.md) — совместимость клиентов.
- [../SECURITY.md](../../SECURITY.md) — модель угроз.
