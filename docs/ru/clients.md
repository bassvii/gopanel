# Совместимость клиентов с ссылками gopanel

**Статус: WIP.** Таблица заполнена частично. Если у вас есть
доступ к непроверенным клиентам — дополните (см. ниже «Как добавить
строку»).

Проверено на версии gopanel: 0.1 (в разработке).
Дата последнего обновления: 2026-10-14.

## Как проверялось

Для каждого клиента:

1. Ссылка сгенерирована через `GET /users/{id}/links?address=<IP>`
   (в UI: «Пользователи → Ссылки»).
2. Импортирована в клиент: вставкой URL или сканированием QR.
3. Проверено подключение (открытие `https://ifconfig.me`).
4. Результат записан в таблицу.

## Результаты

| Клиент | ОС | VLESS | VLESS+WS | VLESS+REALITY | VMess | Trojan | Shadowsocks | Hysteria2 | QR | Ссылка | Примечания |
|---|---|---|---|---|---|---|---|---|---|---|---|
| [v2rayN 7.12.5][v2rayN] | Windows | ✓ | — | — | — | — | — | — | ✓ | ✓ | TUN-режим ломает TLS в WSL — использовать системный прокси |
| Xray CLI | Linux (WSL) | ✓ | — | — | — | — | — | — | — | — | Проверено через SOCKS-клиент |
| [Hiddify][Hiddify] | Windows | — | — | — | — | — | — | — | — | — | |
| [Nekoray][Nekoray] | Windows / Linux | — | — | — | — | — | — | — | — | — | |
| [Streisand][Streisand] | iOS | — | — | — | — | — | — | — | — | — | |
| [v2rayNG][v2rayNG] | Android | — | — | — | — | — | — | — | — | — | |
| [sing-box][sing-box] | Linux / macOS | — | — | — | — | — | — | — | — | — | |
| [Shadowrocket][Shadowrocket] | iOS | — | — | — | — | — | — | — | — | — | Платный |

**Обозначения:**

- **✓** — работает;
- **✗** — не работает;
- **—** — не проверялось.

[v2rayN]: https://github.com/2dust/v2rayN
[Hiddify]: https://github.com/hiddify/hiddify-next
[Nekoray]: https://github.com/MatsuriDayo/nekoray
[Streisand]: https://apps.apple.com/app/streisand/id6450534064
[v2rayNG]: https://github.com/2dust/v2rayNG
[sing-box]: https://github.com/SagerNet/sing-box
[Shadowrocket]: https://apps.apple.com/app/shadowrocket/id932747118

## Что тестировать в первую очередь

Минимальный набор для MVP:

1. **v2rayN** (Windows) — самый популярный десктопный клиент.
2. **Hiddify** (Windows / macOS / Linux / Android / iOS) — современный,
   поддерживает все основные протоколы.
3. **v2rayNG** (Android) — популярный мобильный.
4. **Streisand** или **Shadowrocket** (iOS) — если есть доступ к iPhone.

## Как добавить строку

1. Сгенерируйте ссылку в панели: **Пользователи → Ссылки**.
2. Вставьте в клиент (или отсканируйте QR).
3. Проверьте подключение: откройте `https://ifconfig.me`.
4. Допишите строку в таблицу:

   ```
   | [Имя клиента][ref] | ОС | VLESS | ... | Примечание |
   ```

   Где `ref` — ссылка на скачивание.
5. Добавьте ссылку на клиент в конец файла (после таблицы):
   ```
   [ref]: https://example.com/download
   ```
6. Откройте Pull Request с описанием:
   - версия клиента,
   - ОС,
   - какие протоколы проверены,
   - результат (✓/✗).

## Форматы ссылок

gopanel генерирует:

- **VLESS:**
  ```
  vless://<uuid>@<host>:<port>?<params>#<remark>
  ```
  Параметры: `type` (tcp/ws/grpc/xhttp), `security` (none/tls/reality),
  `sni`, `fp`, `alpn`, `flow`, `pbk` (для REALITY), `sid`, `path`
  (для ws), `host` (для ws), `serviceName` (для grpc), `encryption`
  (VLESS Encryption).

- **VMess:**
  ```
  vmess://<base64-json>
  ```
  JSON: `{v, ps, add, port, id, aid, scy, net, type, host, path, tls, sni, fp, alpn}`.
  Кодируется в base64 (StdEncoding).

- **Trojan:**
  ```
  trojan://<password>@<host>:<port>?<params>#<remark>
  ```
  Параметры — как у VLESS (transport, security).

- **Shadowsocks (SIP002):**
  ```
  ss://<base64(method:password)>@<host>:<port>#<remark>
  ```
  Используется `base64.RawURLEncoding` (без padding).

- **Hysteria2:**
  ```
  hysteria2://<auth>@<host>:<port>/?alpn=h3&sni=<sni>&insecure=1#<remark>
  ```
  `insecure=1` — только если используется самоподписанный сертификат.

## Известные ограничения

### TUN-режим и WSL

TUN-режим в клиентах (v2rayN, sing-box, Hiddify) перехватывает трафик
WSL и ломает TLS/DNS. Используйте системный прокси, не TUN, если
сервер gopanel стоит на WSL.

### VLESS + REALITY

Требует поддержки REALITY на стороне клиента.

- **v2rayN:** с версии 6.x.
- **v2rayNG:** с версии 1.8+.
- **Streisand:** да, поддерживает.
- **Nekoray:** да, поддерживает.
- **sing-box:** да, поддерживает.

### Shadowsocks 2022

Если используете метод `2022-blake3-aes-256-gcm`, клиент должен
поддерживать его. Не все старые клиенты умеют.

Проверено:

- **v2rayN 7.12.5** — да.
- **v2rayNG** — да, с 1.9+.
- **Hiddify** — да.

### IPv6

Адрес в URL должен быть в квадратных скобках:

```
vless://uuid@[2001:db8::1]:443?...
```

gopanel это делает автоматически (`hostForURL` в `internal/links/vless.go`).

### Hysteria2

- Требует **UDP** — проверьте firewall (см. `install.md` и `install-docker.md`).
- Для самоподписанных сертификатов клиент должен принимать `insecure=1`.
- Порт в URL — UDP-порт инбаунда.
- В некоторых клиентах схема называется `hy2://` вместо `hysteria2://`.

### VMess

Формат `vmess://` — base64 от JSON. Некоторые клиенты (особенно
старые) не понимают поля `sni` и `fp` в этом JSON. Если
подключение не проходит — попробуйте импортировать в другой клиент.

## Что не тестировалось

- **XHTTP** транспорт — относительно новый, поддержка зависит от клиента.
- **VLESS Encryption** (`encryption` в URL) — требует свежих клиентов.
- **gRPC** транспорт — проверялся только в v2rayN.
- **HTTPUpgrade** — не проверялся.
- **mKCP** — не проверялся.

## См. также

- [install.md](install.md) — установка панели.
- [install-docker.md](install-docker.md) — установка в Docker.
- [xray-notes.md](xray-notes.md) — заметки по Xray API.
