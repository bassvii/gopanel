# CLI gopanel

Справочник по всем командам `gopanel`.

Все команды запускаются через бинарник:

- `gopanel` — если установлен через `install.sh` (`/usr/local/bin/gopanel`).
- `./bin/gopanel` — при запуске из исходников.

## Обзор

| Команда | Что делает |
|---|---|
| [`gopanel run`](#gopanel-run) | Запустить панель (HTTP + Xray + фоновые задачи) |
| [`gopanel admin create`](#gopanel-admin-create) | Создать администратора |
| [`gopanel reset-password`](#gopanel-reset-password) | Сбросить пароль админа (аварийно) |
| [`gopanel config`](#gopanel-config) | Показать/изменить настройки (порт, base_path) |
| [`gopanel backup`](#gopanel-backup) | Создать дамп базы |

Без аргументов `gopanel` эквивалентна `gopanel run`.

## Порядок приоритетов настроек

Для `run` (и большинства команд):

1. **Флаги** командной строки (`--db`, `--port`, …).
2. **Env** (`GOPANEL_DB_PATH`, `GOPANEL_PORT`, …).
3. **JSON-файл** (`--config /etc/gopanel/config.json`).
4. **Значения по умолчанию** (`Default()`).

Для `--config` отдельный приоритет: `--config` > `GOPANEL_CONFIG` > пусто.

---

## `gopanel run`

Запускает панель: HTTP-listener, Xray, сборщик статистики, retention, лимиты.

### Синтаксис

```
gopanel run [flags]
```

### Флаги

| Флаг | Env | По умолчанию | Что делает |
|---|---|---|---|
| `--config PATH` | `GOPANEL_CONFIG` | — | Путь к JSON-конфигу |
| `--db PATH` | `GOPANEL_DB_PATH` | `./gopanel.db` | Путь к SQLite |
| `--listen ADDR` | `GOPANEL_LISTEN` | `127.0.0.1` | Адрес listener |
| `--port N` | `GOPANEL_PORT` | `0` (случайный) | Порт listener |
| `--base-path PATH` | `GOPANEL_BASE_PATH` | (генерируется) | Секретный префикс URL |
| `--xray-bin PATH` | `GOPANEL_XRAY_BIN` | `/usr/local/bin/xray` | Путь к бинарнику Xray |
| `--xray-config PATH` | `GOPANEL_XRAY_CONFIG` | `/tmp/gopanel-xray.json` | Куда писать конфиг Xray (в systemd-установке — `/run/gopanel/xray.json`) |
| `--log-level LEVEL` | `GOPANEL_LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |
| `--secure-cookies` | `GOPANEL_SECURE_COOKIES=1` | `false` | Secure-флаг на cookie сессии |
| `--trusted-proxies CIDR` | `GOPANEL_TRUSTED_PROXIES` | — | CIDR через запятую |

### Примеры

**Локально, на тестовой базе:**

```bash
./bin/gopanel run --db /tmp/gp.db --port 8443 --log-level debug
```

**В systemd (см. `deploy/install.sh`):**

```
ExecStart=/usr/local/bin/gopanel run \
    --config /etc/gopanel/config.json \
    --db /var/lib/gopanel/gopanel.db
```

**С доверенным прокси:**

```bash
./bin/gopanel run --trusted-proxies "127.0.0.1/32,10.0.0.0/8"
```

---

## `gopanel admin create`

Создать администратора. Пароль вводится **интерактивно** (дважды, без эха). Минимум 12 символов (argon2id).

### Синтаксис

```
gopanel admin create --user NAME [--db PATH] [--config PATH]
```

### Флаги

| Флаг | По умолчанию | Что делает |
|---|---|---|
| `--user NAME` | — | **Обязательно.** Имя администратора |
| `--db PATH` | из конфига | Путь к SQLite |
| `--config PATH` | `GOPANEL_CONFIG` | Путь к JSON-конфигу |

### Пример

```bash
sudo -u gopanel /usr/local/bin/gopanel admin create \
    --db /var/lib/gopanel/gopanel.db \
    --user admin
```

Вывод:

```
пароль для "admin": ********
повторите пароль: ********
admin "admin" created (id=1)
```

### Env-fallback для CI

`GOPANEL_ADMIN_PASSWORD` — если задан, пароль берётся из него (без интерактивного ввода). **Не рекомендуется** в проде: значение видно в `docker inspect`, `systemctl show`, окружении процесса.

### Что удалено

Флаг `--password` **удалён** в v0.1. Раньше он попадал в `ps` и shell history. `--password-stdin` оставлен как заглушка, всегда возвращает ошибку.

---

## `gopanel reset-password`

Аварийный сброс пароля администратора. **Заодно отключает 2FA и завершает все сессии** пользователя.

Пароль вводится интерактивно.

### Синтаксис

```
gopanel reset-password --user NAME [--db PATH] [--config PATH]
```

### Пример

```bash
sudo -u gopanel /usr/local/bin/gopanel reset-password \
    --db /var/lib/gopanel/gopanel.db \
    --user admin
```

Вывод:

```
пароль для "admin": ********
повторите пароль: ********
password reset for "admin"; 2FA disabled; all sessions terminated
```

### Когда использовать

- Забыли пароль админа.
- Потеряли доступ к приложению-аутентификатору и recovery-кодам.
- Подозрение на компрометацию (смена пароля + выкидывание всех сессий).

В `audit_log` пишется действие `password_reset` с IP `cli`.

---

## `gopanel config`

Показать или изменить настройки: **порт** и **base_path**. Оба хранятся в БД (`settings`), а не в файле.

### Синтаксис

```
gopanel config [--port N] [--base-path PATH] [--db PATH] [--config PATH]
```

### Пример — показать текущие

```bash
gopanel config --db ~/gopanel-data/gp.db
```

Вывод:

```
port:      38141
base_path: /cad0682a6c090a4eb555e629e10ec955
db_path:   /home/user/gopanel-data/gp.db
```

### Пример — сменить порт

```bash
gopanel config --db ~/gopanel-data/gp.db --port 8443
# → port set to 8443
# → restart the panel to apply changes
```

### Пример — сменить base_path

```bash
gopanel config --db ~/gopanel-data/gp.db --base-path mysecret
# → base_path set to /mysecret
```

Ведущий `/` добавляется автоматически, если его нет.

### Важно

Изменения вступают в силу **после перезапуска панели**:

```bash
sudo systemctl restart gopanel
```

---

## `gopanel backup`

Создать логический дамп базы. Использует `VACUUM INTO` — безопасно для работающей базы (SQLite делает консистентный снимок).

### Синтаксис

```
gopanel backup [--db PATH] [--out PATH] [--config PATH]
```

### Флаги

| Флаг | По умолчанию | Что делает |
|---|---|---|
| `--db PATH` | из конфига | Путь к SQLite |
| `--out PATH` | рядом с БД, имя `gopanel-YYYY-MM-DD-HHMMSS.db` | Куда сохранить дамп |
| `--config PATH` | `GOPANEL_CONFIG` | Путь к JSON-конфигу |

### Пример

```bash
sudo -u gopanel /usr/local/bin/gopanel backup \
    --db /var/lib/gopanel/gopanel.db \
    --out /var/lib/gopanel/backup-2026-10-14.db
```

Вывод:

```
backup created: /var/lib/gopanel/backup-2026-10-14.db (34816 bytes)
```

Файл создаётся с правами `0600`.

### Что попадает в дамп

Все таблицы:

- `admins` (включая `totp_secret`, **зашифрованный** AES-256-GCM),
- `sessions`, `recovery_codes`,
- `users`, `inbounds`, `user_inbound`, `outbounds`, `routing_rules`,
- `audit_log`, `notifications`, `settings`,
- `traffic` (без агрегации).

Ключ шифрования (`secret.key`) **в дамп не входит**. Если потерять ключ — расшифровать `totp_secret` из бэкапа нельзя.

### Автоматизация

Cron:

```cron
0 3 * * * /usr/local/bin/gopanel backup \
    --db /var/lib/gopanel/gopanel.db \
    --out /backups/gopanel-$(date +\%F).db
30 3 * * * find /backups -name "gopanel-*.db" -mtime +30 -delete
```

Cron запускать от `gopanel` (или с `sudo -u gopanel`), иначе права на файл будут другие.

---

## Глобальные переменные окружения

| Переменная | Эквивалент | Команда |
|---|---|---|
| `GOPANEL_CONFIG` | `--config` | все |
| `GOPANEL_DB_PATH` | `--db` | `run`, `admin create`, `reset-password`, `config`, `backup` |
| `GOPANEL_LISTEN` | `--listen` | `run` |
| `GOPANEL_PORT` | `--port` | `run` |
| `GOPANEL_BASE_PATH` | `--base-path` | `run` |
| `GOPANEL_XRAY_BIN` | `--xray-bin` | `run` |
| `GOPANEL_XRAY_CONFIG` | `--xray-config` | `run` |
| `GOPANEL_LOG_LEVEL` | `--log-level` | `run` |
| `GOPANEL_ALLOW_NON_LOOPBACK=1` | — | `run` (только для Docker) |
| `GOPANEL_SECURE_COOKIES=1` | `--secure-cookies` | `run` |
| `GOPANEL_TRUSTED_PROXIES` | `--trusted-proxies` | `run` |
| `GOPANEL_ADMIN_PASSWORD` | — | `admin create`, `reset-password` (**небезопасно**) |

## Диагностика

**`command not found: gopanel`** — бинарник не в `PATH`. Используйте полный путь `/usr/local/bin/gopanel` или `./bin/gopanel`.

**`flag provided but not defined: -password`** — флаг удалён. См. `admin create`.

**`--config` не читается** — проверьте, что `--config` **перед** остальными флагами (или что env `GOPANEL_CONFIG` задан). Файл должен быть валидным JSON.

**`admin create` не пускает** — пароль короче 12 символов. Минимум — 12.

**`backup: stat ... no such file or directory`** — `--out` в несуществующем каталоге. Создайте каталог заранее.
