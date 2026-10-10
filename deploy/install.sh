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
        log "Xray уже установлен: $($XRAY_BIN version | head -1)"
        return
    fi

    log "устанавливаю Xray-core"

    local tmp
    tmp=$(mktemp -d)
    trap 'rm -rf "$tmp"' RETURN

    local url="${XRAY_REPO}/releases/latest/download/Xray-linux-64.zip"
    if [[ "$ARCH" == "arm64" ]]; then
        url="${XRAY_REPO}/releases/latest/download/Xray-linux-arm64-v8a.zip"
    fi

    curl -fsSL -o "$tmp/xray.zip" "$url"
    unzip -q -o "$tmp/xray.zip" -d "$tmp/xray"

    install -m 0755 "$tmp/xray/xray" "$XRAY_BIN"

    # Даём Xray право занимать порты < 1024 (443 и т.п.).
    setcap 'cap_net_bind_service=+ep' "$XRAY_BIN" 2>/dev/null || \
        warn "setcap не сработал: Xray не сможет занять порты < 1024"

    log "Xray установлен: $($XRAY_BIN version | head -1)"
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

    local src=""
    if [[ -x "./gopanel" ]]; then
        src="./gopanel"
    elif [[ -x "./bin/gopanel" ]]; then
        src="./bin/gopanel"
    else
        die "бинарник gopanel не найден в текущем каталоге. Соберите: go build -o bin/gopanel ./cmd/gopanel"
    fi

    install -m 0755 "$src" "$GOPANEL_BIN"
}

# --- Каталоги и база ---

setup_dirs() {
    log "создаю каталоги"
    install -d -m 0750 -o "$GOPANEL_USER" -g "$GOPANEL_USER" "$GOPANEL_DIR"
    install -d -m 0755 "$GOPANEL_CONFIG_DIR"
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
ExecStart=$GOPANEL_BIN run --db $GOPANEL_DB
Restart=on-failure
RestartSec=5

# Безопасность
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
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
        warn "не удалось извлечь порт и base_path из лога"
        warn "проверьте: journalctl -u gopanel.service -n 50"
        return
    fi

    local port base
    port=$(echo "$log_line" | grep -oP 'addr=127.0.0.1:\K\d+')
    base=$(echo "$log_line" | grep -oP 'base_path=\K/\S+')

    cat <<EOF

${GREEN}Установка завершена.${NC}

  Панель работает на 127.0.0.1:${port}, base_path: ${base}

  Открыть панель (через SSH-туннель):

      ssh -L ${port}:127.0.0.1:${port} user@your-server

  Затем в браузере:

      http://127.0.0.1:${port}${base}/

  Первого админа создайте командой:

      sudo -u gopanel ${GOPANEL_BIN} admin create --db ${GOPANEL_DB} \\
          --user admin --password 'ваш-пароль'

  Или через переменные окружения (для Docker):

      GOPANEL_ADMIN_USER=admin GOPANEL_ADMIN_PASSWORD='пароль'

  Управление сервисом:

      systemctl status gopanel
      systemctl restart gopanel
      systemctl stop gopanel
      journalctl -u gopanel -f

EOF
}

# --- Main ---

main() {
    check_root
    check_os
    check_arch
    install_deps
    install_xray
    create_user
    install_gopanel_bin
    setup_dirs
    init_db
    install_systemd
    start_panel
    print_info
}

main "$@"
