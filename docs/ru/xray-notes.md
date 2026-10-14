# Заметки по Xray-core

Справочник по поведению Xray, проверенному экспериментально.
Используется при разработке gopanel. Не является документацией Xray.

Проверено: Xray 26.3.27, 13 октября 2026.

## Содержание

- [1. Команда проверки конфига](#1-команда-проверки-конфига)
- [2. Счётчики StatsService](#2-счётчики-statsservice)
- [3. Runtime API (hot-reload)](#3-runtime-api-hot-reload)
- [4. Обязателен ли email](#4-обязателен-ли-email)
- [5. Поведение счётчиков при перезапуске](#5-поведение-счётчиков-при-перезапуске)
- [6. Версия и проверка бинарника](#6-версия-и-проверка-бинарника)
- [7. Hysteria2 в Xray-core](#7-hysteria2-в-xray-core)
- [8. Как это отражено в коде gopanel](#8-как-это-отражено-в-коде-gopanel)

---

## 1. Команда проверки конфига

Рабочая команда:

```bash
xray run -test -c /usr/local/etc/xray/config.json
```

**Примечание:**

- `-c` принимает **файл**, `-confdir` — каталог.
- Передача каталога в `-c` даёт ошибку `Failed to get format`.
- Exit code `0` — конфиг валиден.

## 2. Счётчики StatsService

### Формат имён счётчиков

```
# По инбаундам
inbound>>>[tag]>>>traffic>>>uplink
inbound>>>[tag]>>>traffic>>>downlink

# По пользователям
user>>>[email]>>>traffic>>>uplink
user>>>[email]>>>traffic>>>downlink

# Онлайн
user>>>[email]>>>online
```

### Команды

```bash
# Вся статистика
xray api statsquery --server=127.0.0.1:10085

# Только по пользователям
xray api statsquery --server=127.0.0.1:10085 -pattern "user>>>"

# Онлайн-IP конкретного пользователя
xray api statsonlineiplist --server=127.0.0.1:10085 -email "user1@test"

# Число онлайн-сессий
xray api statsonline --server=127.0.0.1:10085 -email "user1@test"

# Все онлайн-пользователи
xray api statsgetallonlineusers --server=127.0.0.1:10085
```

### Наблюдения из экспериментов

- Счётчики заполняются только после **реального трафика** через инбаунд.
- При первом запросе на пустом сервере — `{}` или только `inbound>>>api`.
- После трафика через `vless-in` появились:
  ```
  user>>>user1@test>>>traffic>>>uplink = 1877
  user>>>user1@test>>>traffic>>>downlink = 6464
  ```
- `statsonlineiplist` возвращает `{"name":"user>>>user1@test>>>online"}` **без поля `ips`**, если пользователь уже отключился (онлайн-таблица живёт только на время соединения).
- `statsgetallonlineusers` возвращает `{}` когда никто не подключён.
- **Локальные адреса (`127.0.0.1`, `::1`) игнорируются намеренно.**

## 3. Runtime API (hot-reload)

### Доступные команды в Xray 26.3.27

- `adu` — Add users to inbounds
- `rmu` — Remove users from inbounds
- `inbounduser` — Retrieve inbound user(s)
- `inboundusercount` — Retrieve inbound user count
- `statsonline` — Retrieve the online session count for a user
- `statsonlineiplist` — Retrieve a user's online IP addresses and access times
- `statsgetallonlineusers` — Retrieve array of all online users

**Вывод:** `HotReload=true`, `RuntimeUser=true`.

### Синтаксис `adu`

```
usage: xray api adu [--server=127.0.0.1:8080] <c1.json> [c2.json]...
```

- Принимает **файлы JSON** с описанием инбаундов, **не JSON-строку**.
- Флаг `-inbound` **не существует**.

### Требования `adu`

- В передаваемом файле инбаунд должен содержать **`port` и `listen`**, даже если инбаунд уже работает на сервере.
  - Без порта: `Listen on AnyIP but no Port(s) set in InboundDetour`.
- Пользователь **без `email` молча пропускается**:
  - `Added 0 user(s) in total` без ошибки.
- С `email` добавляется нормально: `Added 1 user(s) in total`.

### Что это значит для панели

- Панель **может** добавлять/удалять пользователей без перезапуска ядра.
- При вызове `adu` нужно передавать **полный инбаунд** с `port` и `listen`.
- `email` обязателен — генерируется панелью для каждого пользователя.

## 4. Обязателен ли email

**ДА, обязателен.**

- `adu` пропускает пользователя без email: `Added 0 user(s) in total`.
- В исходном коде Xray: `if len(user.Email) < 1 { continue }`.
- Без email **нет счётчиков** `user>>>...>>>traffic>>>...`.
- Панель должна генерировать **уникальный email** для каждого пользователя.

## 5. Поведение счётчиков при перезапуске

**Счётчики ОБНУЛЯЮТСЯ при перезапуске ядра.**

Проверка:

- **До перезапуска:**
  ```
  user>>>user1@test>>>traffic>>>uplink = 1877
  user>>>user1@test>>>traffic>>>downlink = 6464
  ```
- **Перезапуск:** Ctrl+C, затем `xray run -c ...` заново.
- **После перезапуска:** счётчики `user>>>...>>>traffic>>>...` отсутствуют (или `0`).

**Причина:** счётчики StatsService живут **в памяти процесса** и не сохраняются на диск.

### Что это значит для панели

- Панель **НЕ может полагаться** на абсолютные значения из Xray.
- Нужно хранить последнее известное значение по каждому пользователю в SQLite и считать **приращение** между опросами.
- При обнаружении уменьшения значения (или исчезновения счётчика) считать это **сбросом**: не вычитать разницу, а принять новое значение как базовое, а разницу не начислять (иначе можно уйти в минус или списать лишнее).

**Алгоритм:**

1. Панель опрашивает StatsService раз в N секунд.
2. Для каждого пользователя: `new_up`, `new_down`.
3. Если `new_up < last_up` — счётчик сброшен, `delta = new_up`. Иначе `delta = new_up - last_up`.
4. `used_bytes += delta`.
5. `last_up = new_up`, `last_down = new_down`.

**Важно:** опрос должен происходить **до** перезапуска ядра, иначе приращение за период между последним опросом и перезапуском будет потеряно. Перед плановым перезапуском ядра панель должна сначала опросить счётчики, сохранить значения, и только потом перезапускать.

## 6. Версия и проверка бинарника

- Бинарник: `/usr/local/bin/xray` (после установки `install.sh`).
- Версия: **26.3.27** (зафиксирована в `deploy/versions.env`).
- Проверка версии: `xray version` → первая строка `Xray 26.3.27 (...)`.
- Проверка конфига: `xray run -test -c <файл>` → exit `0` при успехе.
- Панель вызывает эти команды через `os/exec`.

## 7. Hysteria2 в Xray-core

Проверено 13 октября 2026, Xray 26.3.27.

- **Protocol:** `hysteria`, version 2.
- **Transport:** `streamSettings.network: "hysteria"`.
- **TLS обязателен:** `security: "tls"`, `alpn: ["h3"]`.
- **Сертификат:** `xray tls cert` создаёт самоподписанный.
- **Users:** `settings.users[]` — обязательны `auth` (пароль) и `email`.
- **Порт:** UDP (QUIC). Проверять `ss -ulnp`, не `-tlnp`.
- **StatsService:** `user>>>[email]>>>traffic>>>uplink/downlink` — email обязателен.
- **Команда проверки:** `xray run -test -c ...`.

### Пример конфига

```json
{
  "inbounds": [
    {
      "tag": "hysteria-in",
      "listen": "0.0.0.0",
      "port": 1444,
      "protocol": "hysteria",
      "settings": {
        "version": 2,
        "users": [{ "auth": "testpassword123", "email": "hysteria@test" }]
      },
      "streamSettings": {
        "network": "hysteria",
        "security": "tls",
        "tlsSettings": {
          "certificates": [{ "certificateFile": "...", "keyFile": "..." }],
          "alpn": ["h3"]
        },
        "hysteriaSettings": { "version": 2 }
      }
    }
  ]
}
```

## 8. Как это отражено в коде gopanel

| Знание из заметок | Файл в gopanel | Что там |
|---|---|---|
| Проверка конфига `-test` | `internal/core/xray/xray.go` | `TestConfig()` |
| StatsService, формат `user>>>email>>>traffic>>>...` | `internal/stats/collector.go` | `parseStats()`, `pollOnce()` |
| Счётчики сбрасываются при перезапуске | `internal/stats/collector.go` | Логика `sample.up < last.up` → reset |
| Опрос раз в 30 сек | `internal/stats/collector.go` | `stats.New(..., 30*time.Second, ...)` |
| email обязателен | `internal/api/users.go` | `generateCredential()`, `toUserDTO()` |
| adu/rmu (hot-reload) | **не используется** | См. `PLAN.ru.md` (Этап 6, «Возможности Xray») |
| Hysteria2 config | `internal/core/xray/config.go` | `BuildConfig()` — добавляет `hysteriaSettings` |
| Hysteria2 ссылка | `internal/links/hysteria2.go` | `generateHysteria2()` |
| Версия Xray | `deploy/versions.env` | `XRAY_VERSION`, `XRAY_SHA256_*` |
| Установка Xray | `deploy/install.sh` | `install_xray()` — скачивание + SHA-256 |
| Capability для портов < 1024 | `deploy/install.sh` | `install_systemd()` — `AmbientCapabilities` |

### Что не используется

- **`adu` / `rmu`** (hot-reload пользователей) — панель перезапускает Xray при изменении пользователей. См. `PLAN.ru.md` #11 (Этап 6).
- **`statsonline` / `statsonlineiplist`** — не используются. Лимит устройств (Этап 6) потребует их.
- **`inbounduser` / `inboundusercount`** — не используются.
