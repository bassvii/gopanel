# Бэкапы и восстановление

gopanel хранит всё в одном файле SQLite. Бэкап — это логический
дамп базы через `VACUUM INTO`, который безопасен для работающей панели.

## Что попадает в бэкап

Все таблицы:

- `admins` — администраторы, хеши паролей, **зашифрованные** TOTP-секреты.
- `sessions` — активные сессии (можно не переносить, но проще «как есть»).
- `recovery_codes` — хеши кодов восстановления 2FA.
- `users` — VPN-пользователи, квоты, сроки.
- `inbounds`, `outbounds`, `routing_rules` — конфигурация ядра.
- `user_inbound` — привязки пользователей к инбаундам с credentials.
- `traffic` — история трафика (может быть большой).
- `audit_log`, `notifications` — журналы.
- `settings` — порт админки, base_path.

**Что НЕ попадает:**

- **`secret.key`** — ключ шифрования TOTP-секретов. Лежит в
  `<db-dir>/secret.key`. **Без него восстановленные TOTP-секреты
  расшифровать нельзя** — админ не сможет войти с 2FA, придётся
  сбрасывать.
- **Бинарник `gopanel`** — восстанавливать нечего.
- **Бинарник `xray`** и `geoip.dat`/`geosite.dat` — переустанавливаются
  из `install.sh`.
- **`base_path`** — файл `<db-dir>/base_path`. Есть в БД (в `settings`),
  так что восстанавливается автоматически.

## Создать бэкап

### CLI

```bash
sudo -u gopanel /usr/local/bin/gopanel backup \
    --db /var/lib/gopanel/gopanel.db \
    --out /var/lib/gopanel/backup-$(date +%F).db
```

Файл создаётся с правами `0600`. Если `--out` не указан — имя генерируется
автоматически: `gopanel-YYYY-MM-DD-HHMMSS.db` рядом с БД.

### Через UI

**Настройки → Резервные копии → Скачать бэкап.**

Требует **пароль администратора** (защита от украденной сессии).
Дамп отдаётся через `POST /backup/download`.

### Что записывать в бэкап вместе с дампом

- **`secret.key`** — если не хранится в секретном месте. **Обязательно**
  для восстановления 2FA.
- **Версия панели** (`gopanel version` или коммит).
- **`deploy/versions.env`** — зафиксированные версии Xray/GeoIP.

## Автоматизация

### Cron (systemd-установка)

```bash
sudo crontab -u gopanel -e
```

```cron
0 3 * * * /usr/local/bin/gopanel backup \
    --db /var/lib/gopanel/gopanel.db \
    --out /var/lib/gopanel/backups/$(date +\%F).db
30 3 * * * find /var/lib/gopanel/backups -name "*.db" -mtime +30 -delete
```

Создайте каталог заранее:

```bash
sudo -u gopanel mkdir -p /var/lib/gopanel/backups
```

### Docker

**Способ 1 — cron на хосте:**

```cron
0 3 * * * docker exec gopanel /usr/local/bin/gopanel backup \
    --db /var/lib/gopanel/gopanel.db \
    --out /var/lib/gopanel/backup-auto.db && \
    docker cp gopanel:/var/lib/gopanel/backup-auto.db \
    /backups/gopanel-$(date +\%F).db
```

**Способ 2 — периодический `docker cp` тома:**

Тома Docker хранятся в `/var/lib/docker/volumes/`, но копировать
оттуда напрямую нельзя — база в WAL-режиме. Используйте CLI-команду
`gopanel backup` внутри контейнера, как выше.

## Шифрование бэкапа

Дамп содержит:

- хеши паролей (argon2id — устойчивы к брутфорсу);
- TOTP-секреты (AES-256-GCM — без `secret.key` бесполезны);
- recovery-коды (sha256);
- credentials VPN-пользователей (UUID, пароли — в открытом виде!).

**Если бэкап хранится вне защищённого сервера** (S3, чужой диск) —
шифруйте файл.

### `age` (современный, простой)

```bash
# Генерация ключа (один раз)
age-keygen -o ~/.age-key.txt

# Шифрование
age -r $(cat ~/.age-key.txt | grep public | awk '{print $4}') \
    -o backup-2026-10-14.db.age \
    backup-2026-10-14.db

# Расшифровка
age -d -i ~/.age-key.txt \
    -o backup-2026-10-14.db \
    backup-2026-10-14.db.age
```

### `gpg`

```bash
gpg --symmetric --cipher-algo AES256 backup-2026-10-14.db
# → backup-2026-10-14.db.gpg

gpg --decrypt backup-2026-10-14.db.gpg > backup-2026-10-14.db
```

**Ключ храните вне сервера** (в менеджере паролей, на бумаге). Иначе
шифрование бесполезно.

## Восстановление

### Systemd-установка

```bash
# 1. Остановить панель
sudo systemctl stop gopanel

# 2. Сделать снимок текущей базы (на случай отката)
sudo cp /var/lib/gopanel/gopanel.db \
    /var/lib/gopanel/gopanel.db.before-restore-$(date +%F)

# 3. Заменить базу
sudo cp backup-2026-10-14.db /var/lib/gopanel/gopanel.db
sudo chown gopanel:gopanel /var/lib/gopanel/gopanel.db
sudo chmod 0600 /var/lib/gopanel/gopanel.db

# 4. Удалить старые WAL-файлы (иначе SQLite подхватит старые данные)
sudo rm -f /var/lib/gopanel/gopanel.db-wal /var/lib/gopanel/gopanel.db-shm

# 5. Запустить
sudo systemctl start gopanel

# 6. Проверить
sudo systemctl status gopanel
sudo journalctl -u gopanel -n 20
```

**Важно:** шаг 4 — удалить `-wal` и `-shm`. Без этого SQLite
может «склеить» старый WAL с новой базой и получить битые данные.

### Docker

**Через существующий контейнер:**

```bash
# 1. Остановить
docker stop gopanel

# 2. Скопировать бэкап внутрь
docker cp backup-2026-10-14.db gopanel:/var/lib/gopanel/gopanel.db

# 3. Удалить WAL-файлы (внутри контейнера)
docker run --rm -v gopanel-data:/data alpine \
    sh -c 'rm -f /data/gopanel.db-wal /data/gopanel.db-shm'

# 4. Запустить
docker start gopanel

# 5. Логи
docker logs gopanel
```

**Через том** (если контейнер удалён):

```bash
# Временный контейнер с тем же томом
docker run --rm -it \
    -v gopanel-data:/data \
    -v $(pwd):/backup \
    alpine sh

# Внутри:
cp /backup/backup-2026-10-14.db /data/gopanel.db
rm -f /data/gopanel.db-wal /data/gopanel.db-shm
chown 999:999 /data/gopanel.db
exit
```

`999:999` — UID/GID пользователя `gopanel` внутри контейнера.
Проверить: `docker exec gopanel id gopanel`.

### Восстановление `secret.key`

Если бэкап содержит **новые** TOTP-секреты (после смены 2FA), а
`secret.key` потерян — расшифровать их нельзя. Придётся:

```bash
sudo -u gopanel /usr/local/bin/gopanel reset-password \
    --db /var/lib/gopanel/gopanel.db \
    --user admin
```

Это сбросит пароль, отключит 2FA и завершит все сессии.

**Поэтому** `secret.key` надо хранить вместе с дампом.

## Проверка бэкапа

Перед тем как полагаться на бэкап, **проверьте его**:

```bash
# 1. Целостность SQLite
sqlite3 backup-2026-10-14.db "PRAGMA integrity_check;"
# → ok

# 2. Таблицы на месте
sqlite3 backup-2026-10-14.db ".tables"
# → admins, audit_log, inbounds, ... (полный список)

# 3. Админ есть
sqlite3 backup-2026-10-14.db "SELECT id, username FROM admins;"
# → 1|admin

# 4. Версия схемы
sqlite3 backup-2026-10-14.db "SELECT MAX(version) FROM schema_migrations;"
# → 12 (или больше)
```

**Раз в месяц** проверяйте восстановление на тестовой машине:

1. Разверните бэкап на чистом сервере.
2. Запустите панель.
3. Убедитесь, что пользователи, инбаунды, 2FA — на месте.

Бэкап, который никогда не восстанавливали, — это не бэкап.

## Внешние хранилища

### S3-совместимое

```bash
# Загрузить
aws s3 cp backup-2026-10-14.db s3://my-bucket/gopanel/

# Скачать
aws s3 cp s3://my-bucket/gopanel/backup-2026-10-14.db .

# Или через rclone (Yandex, Google, Backblaze, ...)
rclone copy backup-2026-10-14.db remote:gopanel-backups/
```

### SFTP

```bash
scp backup-2026-10-14.db user@backup-server:/backups/
```

### Синхронизация

```bash
# Только в одну сторону, удалять лишнее на приёмнике
rsync -av --delete /var/lib/gopanel/backups/ \
    user@backup-server:/backups/gopanel/
```

## Что не восстанавливается из бэкапа

- **Xray-бинарник** — `install.sh` поставит заново.
- **`geoip.dat`/`geosite.dat`** — скачиваются автоматически.
- **Capabilities / systemd-unit** — настраиваются `install.sh`.
- **`secret.key`** — если потерян, TOTP-секреты не расшифруются.

## Быстрая справка

| Задача | Команда |
|---|---|
| Создать бэкап (CLI) | `gopanel backup --db ... --out ...` |
| Создать бэкап (UI) | Настройки → Резервные копии |
| Восстановить (systemd) | `systemctl stop` → `cp` → `rm -wal/-shm` → `systemctl start` |
| Восстановить (Docker) | `docker stop` → `docker cp` → `docker start` |
| Проверить целостность | `sqlite3 backup.db "PRAGMA integrity_check;"` |
| Зашифровать | `age -r ... -o backup.age backup.db` |
| Расшифровать | `age -d -i key.txt -o backup.db backup.age` |
| Если потеряли `secret.key` | `gopanel reset-password --user admin` |

## См. также

- [install.md](install.md) — установка и первый запуск.
- [install-docker.md](install-docker.md) — установка в Docker.
- [cli.md](cli.md) — все команды `gopanel`.
