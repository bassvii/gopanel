#!/usr/bin/env bash
#
# uninstall.sh — удаление gopanel с сервера.
# Запуск: sudo bash uninstall.sh
#
set -euo pipefail

GOPANEL_USER="gopanel"
GOPANEL_BIN="/usr/local/bin/gopanel"
GOPANEL_DIR="/var/lib/gopanel"
GOPANEL_CONFIG_DIR="/etc/gopanel"
SYSTEMD_UNIT="/etc/systemd/system/gopanel.service"

RED=$'\033[0;31m'
GREEN=$'\033[0;32m'
YELLOW=$'\033[1;33m'
NC=$'\033[0m'

log()  { echo -e "${GREEN}==>${NC} $*"; }
warn() { echo -e "${YELLOW}warning:${NC} $*" >&2; }
die()  { echo -e "${RED}error:${NC} $*" >&2; exit 1; }

if [[ $EUID -ne 0 ]]; then
    die "uninstall.sh должен запускаться от root (используйте sudo)"
fi

# Спросить про удаление данных.
KEEP_DATA=false
if [[ -d "$GOPANEL_DIR" ]]; then
    echo
    echo "Папка данных: $GOPANEL_DIR"
    echo "В ней: база данных gopanel.db со всеми пользователями и инбаундами."
    echo
    read -rp "Удалить данные? [y/N]: " answer
    case "$answer" in
        [yY]|[yY][eE][sS]) KEEP_DATA=false ;;
        *) KEEP_DATA=true ;;
    esac
fi

# 1. Остановить сервис.
if systemctl list-unit-files | grep -q "^gopanel.service"; then
    log "останавливаю gopanel.service"
    systemctl stop gopanel.service || warn "не удалось остановить сервис"
    systemctl disable gopanel.service || warn "не удалось отключить сервис"
fi

# 2. Убить оставшиеся процессы.
if pgrep -f "$GOPANEL_BIN" >/dev/null; then
    log "завершаю процессы gopanel"
    pkill -f "$GOPANEL_BIN" || true
    sleep 1
fi

# 3. Удалить systemd unit.
if [[ -f "$SYSTEMD_UNIT" ]]; then
    log "удаляю $SYSTEMD_UNIT"
    rm -f "$SYSTEMD_UNIT"
    systemctl daemon-reload
fi

# 4. Удалить бинарник.
if [[ -f "$GOPANEL_BIN" ]]; then
    log "удаляю $GOPANEL_BIN"
    rm -f "$GOPANEL_BIN"
fi

# 5. Удалить конфиг.
if [[ -d "$GOPANEL_CONFIG_DIR" ]]; then
    log "удаляю $GOPANEL_CONFIG_DIR"
    rm -rf "$GOPANEL_CONFIG_DIR"
fi

# 6. Удалить данные.
if [[ "$KEEP_DATA" == "false" && -d "$GOPANEL_DIR" ]]; then
    log "удаляю данные $GOPANEL_DIR"
    rm -rf "$GOPANEL_DIR"
else
    warn "данные сохранены: $GOPANEL_DIR"
fi

# 7. Удалить пользователя.
if id "$GOPANEL_USER" &>/dev/null; then
    log "удаляю пользователя $GOPANEL_USER"
    userdel "$GOPANEL_USER" 2>/dev/null || warn "не удалось удалить пользователя"
fi

# 8. Xray НЕ удаляем — он может использоваться другими приложениями.
if [[ -x /usr/local/bin/xray ]]; then
    warn "Xray оставлен: /usr/local/bin/xray"
    warn "удалить вручную: sudo rm /usr/local/bin/xray"
fi

log "удаление завершено"
