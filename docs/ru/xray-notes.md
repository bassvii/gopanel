## 1. Команда проверки конфига
Рабочая команда:
xray run -test -c /usr/local/etc/xray/config.json

Примечание: флаг -c принимает файл, -confdir — каталог.
Передача каталога в -c даёт ошибку "Failed to get format".

## 2. Счётчики StatsService

### Формат имён счётчиков
- По инбаундам: inbound>>>[tag]>>>traffic>>>uplink
                inbound>>>[tag]>>>traffic>>>downlink
- По пользователям: user>>>[email]>>>traffic>>>uplink
                    user>>>[email]>>>traffic>>>downlink
- Онлайн: user>>>[email]>>>online

### Команды
Запрос всей статистики:
xray api statsquery --server=127.0.0.1:10085

Запрос по пользователям:
xray api statsquery --server=127.0.0.1:10085 -pattern "user>>>"

Онлайн-IP конкретного пользователя:
xray api statsonlineiplist --server=127.0.0.1:10085 -email "user1@test"

Число онлайн-сессий:
xray api statsonline --server=127.0.0.1:10085 -email "user1@test"

Все онлайн-пользователи:
xray api statsgetallonlineusers --server=127.0.0.1:10085

### Наблюдения из экспериментов
- Счётчики заполняются только после реального трафика через инбаунд.
- При первом запросе на пустом сервере — {} или только inbound>>>api.
- После трафика через vless-in появились:
  user>>>user1@test>>>traffic>>>uplink = 1877
  user>>>user1@test>>>traffic>>>downlink = 6464
- statsonlineiplist возвращает {"name":"user>>>user1@test>>>online"} без поля ips,
  если пользователь уже отключился (онлайн-таблица живёт только на время соединения).
- statsgetallonlineusers возвращает {} когда никто не подключён.
- Локальные адреса (127.0.0.1, ::1) игнорируются намеренно.

## 3. Runtime API (hot-reload)

### Доступные команды в версии Xray 26.3.27
- adu — Add users to inbounds
- rmu — Remove users from inbounds
- inbounduser — Retrieve inbound user(s)
- inboundusercount — Retrieve inbound user count
- statsonline — Retrieve the online session count for a user
- statsonlineiplist — Retrieve a user's online IP addresses and access times
- statsgetallonlineusers — Retrieve array of all online users

Вывод: HotReload=true, RuntimeUser=true.

### Синтаксис adu
usage: xray api adu [--server=127.0.0.1:8080] <c1.json> [c2.json]...

Принимает ФАЙЛЫ JSON с описанием инбаундов, не JSON-строку.
Флаг -inbound не существует.

### Требования adu
- В передаваемом файле инбаунд должен содержать port и listen,
  даже если инбаунд уже работает на сервере.
  Без порта: "Listen on AnyIP but no Port(s) set in InboundDetour".
- Пользователь без email МОЛЧА пропускается:
  "Added 0 user(s) in total" без ошибки.
- С email добавляется нормально: "Added 1 user(s) in total".

### Что это значит для панели
- Панель может добавлять/удалять пользователей без перезапуска ядра.
- При вызове adu нужно передавать полный инбаунд с port и listen.
- email обязателен — генерируется панелью для каждого пользователя.

## 4. Обязателен ли email
ДА, обязателен.
- `adu` пропускает пользователя без email: "Added 0 user(s) in total".
- В исходном коде Xray: `if len(user.Email) < 1 { continue }`.
- Без email нет счётчиков `user>>>...>>>traffic>>>...`.
- Панель должна генерировать уникальный email для каждого пользователя.

## 5. Поведение счётчиков при перезапуске

Счётчики ОБНУЛЯЮТСЯ при перезапуске ядра.

Проверка:
- До перезапуска: user>>>user1@test>>>traffic>>>uplink = 1877
                   user>>>user1@test>>>traffic>>>downlink = 6464
- Перезапуск: Ctrl+C, затем `xray run -c ...` заново.
- После перезапуска: счётчики user>>>...>>>traffic>>>... отсутствуют (или 0).

Причина: счётчики StatsService живут в памяти процесса и не сохраняются на диск.

Что это значит для панели:
- Панель НЕ может полагаться на абсолютные значения из Xray.
- Нужно хранить последнее известное значение по каждому пользователю в SQLite
  и считать ПРИРАЩЕНИЕ между опросами.
- При обнаружении уменьшения значения (или исчезновения счётчика) считать это
  сбросом: не вычитать разницу, а принять новое значение как базовое,
  а разницу не начислять (иначе можно уйти в минус или списать лишнее).
- Алгоритм:
    1. Панель опрашивает StatsService раз в N секунд.
    2. Для каждого пользователя: new_up, new_down.
    3. Если new_up < last_up — счётчик сброшен, delta = new_up.
       Иначе delta = new_up - last_up.
    4. used_bytes += delta.
    5. last_up = new_up, last_down = new_down.
- Опрос должен происходить до перезапуска ядра, иначе приращение за период
  между последним опросом и перезапуском будет потеряно.
- Перед плановым перезапуском ядра панель должна сначала опросить счётчики,
  сохранить значения, и только потом перезапускать.

## 6. Версия и проверка бинарника

- Бинарник: /usr/local/bin/xray (после установки XTLS-скриптом).
- Версия: 26.3.27 (см. выше).
- Проверка версии: `xray version` → первая строка "Xray 26.3.27 (...)".
- Проверка конфига: `xray run -test -c <файл>` → exit 0 при успехе.
- Панель вызывает эти команды через os/exec.
