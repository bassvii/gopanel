// SPDX-License-Identifier: AGPL-3.0-or-later

// Package core определяет общий интерфейс для ядер (Xray, sing-box).
// Панель общается с ядром только через этот интерфейс, чтобы второе
// ядро добавлялось без переписывания панели.
package core

// Traffic — счётчики трафика одного пользователя.
type Traffic struct {
	Up   int64
	Down int64
}

// Capabilities описывает, что умеет активное ядро (или его сборка).
type Capabilities struct {
	UserTraffic bool // счётчики трафика по пользователям
	OnlineIPs   bool // список онлайн-IP по пользователю
	HotReload   bool // применение конфига без перезапуска процесса
	RuntimeUser bool // добавление/удаление пользователей на лету
}

// Core — абстракция над прокси-ядром.
type Core interface {
	BuildConfig() ([]byte, error)
	Validate(cfg []byte) error
	Start() error
	Stop() error
	Reload(cfg []byte) error
	UserTraffic() (map[string]Traffic, error)
	OnlineIPs() (map[string][]string, error)
	Capabilities() Capabilities
}
