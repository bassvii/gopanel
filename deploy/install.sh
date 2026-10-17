#!/usr/bin/env bash
#
# install.sh — установка gopanel на Linux-сервер.
# Запуск: sudo bash install.sh
#
set -euo pipefail

# --- Константы ---

GOPANEL_USER="gopanel"
GOPANEL_BIN="/usr/local/bin/gopanel"
GOPANEL_DIR="/var/lib/gopanel"
GOPANEL_DB="${GOPANEL_DIR}/gopanel.db"
GOPANEL_CONFIG_DIR="/etc/gopanel"
GOPANEL_CONFIG="${GOPANEL_CONFIG_DIR}/config.json"
XRAY_BIN="/usr/local/bin/xray"
SYSTEMD_UNIT="/etc/systemd/system/gopanel.service"

GOPANEL_REPO="${GOPANEL_REPO:-https://github.com/bassvii/gopanel}"
XRAY_REPO="https://github.com/XTLS/Xray-core"
# Geo-файлы хранятся рядом с данными панели, чтобы Xray мог их
# читать, а панель — обновлять через UI (P2-1 третьего аудита).
# Xray получает этот путь через XRAY_LOCATION_ASSET, который
# выставляет панель при запуске.
GEO_DIR="${GOPANEL_DIR}/xray"

# Версии и хеши Xray — в одном файле рядом со скриптом.
# Если файла нет — install.sh отказывается работать:
# без хешей мы не можем гарантировать подлинность бинарника.
VERSIONS_FILE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/versions.env"
if [[ ! -r "$VERSIONS_FILE" ]]; then
    echo "error: $VERSIONS_FILE не найден или недоступен" >&2
    echo "       ожидается рядом с install.sh в deploy/" >&2
    exit 1
fi
# shellcheck source=/dev/null
source "$VERSIONS_FILE"

: "${XRAY_VERSION:?XRAY_VERSION не задан в versions.env}"
: "${XRAY_SHA256_AMD64:?XRAY_SHA256_AMD64 не задан в versions.env}"
: "${XRAY_SHA256_ARM64:?XRAY_SHA256_ARM64 не задан в versions.env}"

# --- Цвета ---

RED=$'\033[0;31m'
GREEN=$'\033[0;32m'
YELLOW=$'\033[1;33m'
NC=$'\033[0m'

log()  { echo -e "${GREEN}==>${NC} $*"; }
warn() { echo -e "${YELLOW}warning:${NC} $*" >&2; }
die()  { echo -e "${RED}error:${NC} $*" >&2; exit 1; }

# --- Проверка окружения ---

check_root() {
    if [[ $EUID -ne 0 ]]; then
        die "install.sh должен запускаться от root (используйте sudo)"
    fi
}

check_os() {
    if [[ ! -f /etc/os-release ]]; then
        die "не удалось определить ОС (нет /etc/os-release)"
    fi
    . /etc/os-release
    case "$ID" in
        debian|ubuntu) ;;
        *) warn "поддерживаются Debian/Ubuntu, продолжаем на $ID" ;;
    esac
}

check_arch() {
    local arch
    arch=$(uname -m)
    case "$arch" in
        x86_64|amd64) ARCH="amd64" ;;
        aarch64|arm64) ARCH="arm64" ;;
        *) die "неподдерживаемая архитектура: $arch" ;;
    esac
}

# --- Зависимости ---

install_deps() {
    log "устанавливаю зависимости (curl, unzip, sqlite3)"
    apt-get update -qq
    apt-get install -y -qq curl unzip sqlite3 ca-certificates >/dev/null
}

# --- Xray ---

install_xray() {
    if [[ -x "$XRAY_BIN" ]]; then
        local installed
        installed=$("$XRAY_BIN" version 2>/dev/null | head -1 || echo "unknown")
        log "Xray уже установлен: $installed"
        log "ожидаемая версия: ${XRAY_VERSION}"

        # Проверяем версию. Если не совпадает с versions.env —
        # предлагаем переустановить (или падаем, если --non-interactive).
        if ! echo "$installed" | grep -q "Xray ${XRAY_VERSION#v}"; then
            warn "установленная версия Xray не совпадает с ${XRAY_VERSION}"
            if [[ "${GOPANEL_NON_INTERACTIVE:-0}" == "1" ]]; then
                die "запустите с обновлённым Xray вручную или удалите $XRAY_BIN"
            fi
            read -rp "Переустановить Xray из официального релиза? [y/N]: " ans
            case "$ans" in
                [yY]|[yY][eE][sS])
                    rm -f "$XRAY_BIN"
                    install_xray
                    return
                    ;;
                *)
                    warn "оставляем текущий Xray без проверки"
                    ;;
            esac
        fi

        # Снимаем возможный старый setcap, если он остался от прошлых версий.
        setcap -r "$XRAY_BIN" 2>/dev/null || true
    else
        log "устанавливаю Xray-core ${XRAY_VERSION}"

        local tmp
        tmp=$(mktemp -d)
        trap 'rm -rf "$tmp"' RETURN

        # URL и ожидаемый хеш зависят от архитектуры.
        # Версия зафиксирована в versions.env, не latest.
        local url expected_sha
        if [[ "$ARCH" == "arm64" ]]; then
            url="${XRAY_REPO}/releases/download/${XRAY_VERSION}/Xray-linux-arm64-v8a.zip"
            expected_sha="$XRAY_SHA256_ARM64"
        else
            url="${XRAY_REPO}/releases/download/${XRAY_VERSION}/Xray-linux-64.zip"
            expected_sha="$XRAY_SHA256_AMD64"
        fi

        log "скачиваю $url"
        curl -fsSL --proto '=https' --tlsv1.2 -o "$tmp/xray.zip" "$url"

        # Проверка контрольной суммы ДО распаковки.
        # При несовпадении — ничего не устанавливаем.
        log "проверяю SHA-256"
        if ! echo "${expected_sha}  $tmp/xray.zip" | sha256sum -c - >/dev/null 2>&1; then
            actual_sha=$(sha256sum "$tmp/xray.zip" | awk '{print $1}')
            die "SHA-256 не совпал!
       ожидалось: ${expected_sha}
       получено:  ${actual_sha}
       Файл повреждён или подменён. Установка прервана."
        fi

        unzip -q -o "$tmp/xray.zip" -d "$tmp/xray"

        install -m 0755 "$tmp/xray/xray" "$XRAY_BIN"

        # setcap здесь НЕ нужен. Xray запускается панелью, а панель
        # работает под systemd-юнитом с AmbientCapabilities=CAP_NET_BIND_SERVICE
        # (см. install_systemd). Capability наследуется дочерним процессом Xray.
        # При NoNewPrivileges=true старый setcap всё равно не сработал бы
        # (см. пункт 4 аудита).

        log "Xray установлен: $($XRAY_BIN version | head -1)"
    fi

    # Проверка, что бинарник Xray отвечает.
    if "$XRAY_BIN" version >/dev/null 2>&1; then
        log "Xray поддерживает Hysteria2 (QUIC включён в официальных сборках)"
    else
        warn "Xray не отвечает на version — проверьте установку"
    fi

    # Скачиваем GeoIP/GeoSite из зафиксированного релиза.
    log "скачиваю GeoIP/GeoSite (тег ${GEO_TAG})"
    mkdir -p "$GEO_DIR"

    : "${GEO_TAG:?GEO_TAG не задан в versions.env}"
    : "${GEO_SHA256_GEOSITE:?GEO_SHA256_GEOSITE не задан в versions.env}"
    : "${GEO_SHA256_GEOIP:?GEO_SHA256_GEOIP не задан в versions.env}"

    local geo_base="https://github.com/Loyalsoldier/v2ray-rules-dat/releases/download/${GEO_TAG}"

    download_geo() {
        local name="$1" url="$2" expected="$3" dst="$4"
        local tmp="${dst}.tmp"

        if ! curl -fsSL --proto '=https' --tlsv1.2 -o "$tmp" "$url"; then
            warn "не удалось скачать ${name} (${url})"
            rm -f "$tmp"
            return 1
        fi

        if ! echo "${expected}  $tmp" | sha256sum -c - >/dev/null 2>&1; then
            local actual
            actual=$(sha256sum "$tmp" | awk '{print $1}')
            warn "${name}: SHA-256 не совпал"
            warn "  ожидалось: ${expected}"
            warn "  получено:  ${actual}"
            rm -f "$tmp"
            return 1
        fi

        chmod 0644 "$tmp"
        mv "$tmp" "$dst"
        return 0
    }

    download_geo "geosite.dat" "${geo_base}/geosite.dat" \
        "$GEO_SHA256_GEOSITE" "$GEO_DIR/geosite.dat" || true

    download_geo "geoip.dat" "${geo_base}/geoip.dat" \
        "$GEO_SHA256_GEOIP" "$GEO_DIR/geoip.dat" || true
}

# --- Firewall ---

# open_udp_for_hysteria — открывает UDP 443 и 8443 в ufw.
# ВЫКЛЮЧЕНО по умолчанию: см. P2-3 третьего аудита.
# На хосте с включённым ufw это открывало бы порты в интернет
# всем, даже если Hysteria2 не используется. Админ должен
# открыть порты сам, когда заведёт инбаунд.
#
# Если нужна автоматика — вызывайте с флагом --open-firewall
# (переменная GOPANEL_OPEN_FIREWALL=1).
open_udp_for_hysteria() {
    if [[ "${GOPANEL_OPEN_FIREWALL:-0}" != "1" ]]; then
        return 0
    fi
    if command -v ufw >/dev/null 2>&1 && ufw status 2>/dev/null | grep -q "Status: active"; then
        log "открываю UDP-порты для Hysteria2 в ufw (GOPANEL_OPEN_FIREWALL=1)"
        ufw allow 443/udp >/dev/null 2>&1 || true
        ufw allow 8443/udp >/dev/null 2>&1 || true
        ufw reload >/dev/null 2>&1 || true
    fi
}

# --- Пользователь ---

create_user() {
    if id "$GOPANEL_USER" &>/dev/null; then
        log "пользователь $GOPANEL_USER уже существует"
        return
    fi
    log "создаю системного пользователя $GOPANEL_USER"
    useradd --system --no-create-home --shell /usr/sbin/nologin \
        --home-dir "$GOPANEL_DIR" "$GOPANEL_USER"
}

# --- Бинарник панели ---

install_gopanel_bin() {
    log "устанавливаю бинарник панели"

    # Берём только свежесобранный bin/gopanel. Файл ./gopanel в корне
    # репозитория больше не используется: раньше он лежал в git и мог
    # не совпадать с текущими исходниками (пункт 5 аудита).
    if [[ ! -x "./bin/gopanel" ]]; then
        die "бинарник ./bin/gopanel не найден. Соберите: make build"
    fi

    install -m 0755 "./bin/gopanel" "$GOPANEL_BIN"
}

# --- Каталоги и база ---

setup_dirs() {
    log "создаю каталоги"
    install -d -m 0750 -o "$GOPANEL_USER" -g "$GOPANEL_USER" "$GOPANEL_DIR"
    install -d -m 0755 "$GOPANEL_CONFIG_DIR"
    # Geo-файлы рядом с данными, владелец — gopanel, чтобы
    # панель могла их обновлять через UI (P2-1).
    install -d -m 0755 -o "$GOPANEL_USER" -g "$GOPANEL_USER" "$GEO_DIR"

    # Пустой конфиг: панель стартует с дефолтами, всё можно
    # переопределить через env или флаги. Класть сюда db_path/listen
    # не обязательно — они всё равно переопределяются из --db
    # в ExecStart и из env.
    if [[ ! -f "$GOPANEL_CONFIG" ]]; then
        cat > "$GOPANEL_CONFIG" <<'JSON'
{
  "listen": "127.0.0.1",
  "log_level": "info"
}
JSON
        chmod 0644 "$GOPANEL_CONFIG"
    fi
}

init_db() {
    log "инициализирую базу данных"
    if [[ -f "$GOPANEL_DB" ]]; then
        warn "база уже существует: $GOPANEL_DB — пропускаю"
        return
    fi
    sudo -u "$GOPANEL_USER" "$GOPANEL_BIN" config --db "$GOPANEL_DB" >/dev/null || true
    chown "$GOPANEL_USER:$GOPANEL_USER" "$GOPANEL_DB"
    chmod 0600 "$GOPANEL_DB"
}

# --- Systemd unit ---

install_systemd() {
    log "устанавливаю systemd unit"
    cat > "$SYSTEMD_UNIT" <<EOF
[Unit]
Description=gopanel — self-hosted proxy management panel
After=network.target

[Service]
Type=simple
User=$GOPANEL_USER
Group=$GOPANEL_USER
Environment=GOPANEL_GEO_DIR=$GEO_DIR
ExecStart=$GOPANEL_BIN run --config $GOPANEL_CONFIG --db $GOPANEL_DB --xray-config /run/gopanel/xray.json
Restart=on-failure
RestartSec=5

# Безопасность.
#
# NoNewPrivileges=true не мешает наследованию ambient capabilities:
# они сохраняются при execve, если уже есть в bounding set.
# Проверено на ядре 4.3+. Если Xray не может занять 443 —
# временно верни false и посмотри journalctl.
NoNewPrivileges=true
AmbientCapabilities=CAP_NET_BIND_SERVICE
CapabilityBoundingSet=CAP_NET_BIND_SERVICE

# Песочница systemd.
PrivateTmp=true
PrivateDevices=true
ProtectSystem=strict
ProtectHome=true
ProtectKernelTunables=true
ProtectKernelModules=true
ProtectControlGroups=true
RestrictAddressFamilies=AF_INET AF_INET6 AF_UNIX
RestrictNamespaces=true
LockPersonality=true
UMask=0077
RuntimeDirectory=gopanel
RuntimeDirectoryMode=0700
ReadWritePaths=$GOPANEL_DIR

# Логи
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
EOF

    systemctl daemon-reload
    systemctl enable gopanel.service
}

# --- Запуск ---

start_panel() {
    log "запускаю панель"
    systemctl restart gopanel.service
    sleep 2
    if ! systemctl is-active --quiet gopanel.service; then
        journalctl -u gopanel.service -n 30 --no-pager
        die "панель не запустилась"
    fi
}

# --- Вывод ---

print_info() {
    local log_line
    log_line=$(journalctl -u gopanel.service --no-pager | grep "admin listener started" | tail -1)

    if [[ -z "$log_line" ]]; then
        warn "не удалось извлечь порт из лога"
        warn "проверьте: journalctl -u gopanel.service -n 50"
        return
    fi

    local port base
    port=$(echo "$log_line" | grep -oP 'addr=127.0.0.1:\K\d+')

    # base_path читаем из файла рядом с БД (см. пункт 7 аудита).
    local base_file="${GOPANEL_DIR}/base_path"
    if [[ -r "$base_file" ]]; then
        base=$(cat "$base_file")
    else
        warn "файл $base_file недоступен, base_path не показан"
        base=""
    fi

    cat <<EOF

${GREEN}Установка завершена.${NC}

  Панель работает на 127.0.0.1:${port}, base_path: ${base}
  Xray: $("$XRAY_BIN" version 2>/dev/null | head -1 || echo "не установлен")

  Открыть панель (через SSH-туннель):

      ssh -L ${port}:127.0.0.1:${port} user@your-server

  Затем в браузере:

      http://127.0.0.1:${port}${base}/

  Первого админа создайте командой (спросит пароль интерактивно):

        sudo -u gopanel ${GOPANEL_BIN} admin create --db ${GOPANEL_DB} \\
            --user admin

  Управление сервисом:

      systemctl status gopanel
      systemctl restart gopanel
      systemctl stop gopanel
      journalctl -u gopanel -f

 UDP-порты для Hysteria2 (откройте сами, когда заведёте инбаунд):

       sudo ufw allow 443/udp
       sudo ufw allow 8443/udp

   Повторный запуск install.sh с GOPANEL_OPEN_FIREWALL=1 откроет
   их автоматически (не рекомендуется на общих хостах).

  Если Xray не может занять порт 443 (bind: permission denied):

      journalctl -u gopanel -n 50

  Проверьте, что unit содержит AmbientCapabilities=CAP_NET_BIND_SERVICE
  и NoNewPrivileges=true:

      systemctl show gopanel | grep -E 'AmbientCap|NoNewPriv'

  После правки unit:

      sudo systemctl daemon-reload
      sudo systemctl restart gopanel

EOF
}

# --- Main ---

main() {
    check_root
    check_os
    check_arch
    install_deps
    install_xray
    open_udp_for_hysteria
    create_user
    install_gopanel_bin
    setup_dirs
    init_db
    install_systemd
    start_panel
    print_info
}

main "$@"
