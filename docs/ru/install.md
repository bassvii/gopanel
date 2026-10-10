# Установка gopanel

## Требования

- Linux (Debian 12+, Ubuntu 22.04+)
- root-доступ
- x86_64 или arm64

## Установка

Одна команда:

    curl -fsSL https://raw.githubusercontent.com/bassvii/gopanel/main/deploy/install.sh | sudo bash

Или вручную:

    git clone https://github.com/bassvii/gopanel
    cd gopanel
    # Соберите бинарник (см. README)
    sudo bash deploy/install.sh

Скрипт:

1. Проверит и установит Xray-core (если его нет).
2. Создаст системного пользователя `gopanel`.
3. Установит бинарник в `/usr/local/bin/gopanel`.
4. Инициализирует базу `/var/lib/gopanel/gopanel.db`.
5. Создаст и запустит systemd-сервис `gopanel`.
6. Напечатает URL панели.

## Доступ к панели

Панель слушает только `127.0.0.1`. Доступ — через SSH-туннель:

    ssh -L PORT:127.0.0.1:PORT user@server

Затем в браузере:

    http://127.0.0.1:PORT/BASE_PATH/

`PORT` и `BASE_PATH` напечатаны при установке и в `journalctl -u gopanel`.

## Первый админ

    sudo -u gopanel /usr/local/bin/gopanel admin create \
        --db /var/lib/gopanel/gopanel.db \
        --user admin --password 'ваш-пароль'

## Управление

    systemctl status gopanel
    systemctl restart gopanel
    systemctl stop gopanel
    journalctl -u gopanel -f

## Удаление

    sudo bash deploy/uninstall.sh

Или вручную:

    systemctl stop gopanel
    systemctl disable gopanel
    rm /etc/systemd/system/gopanel.service
    rm /usr/local/bin/gopanel
    rm -rf /var/lib/gopanel /etc/gopanel
    userdel gopanel
