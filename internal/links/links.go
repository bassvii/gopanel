// SPDX-License-Identifier: AGPL-3.0-or-later

// Package links генерирует ссылки протоколов и QR-коды.
package links

import (
	"encoding/json"
	"fmt"
)

// Input — данные, нужные для генерации ссылки.
type Input struct {
	Protocol       string // vless, vmess, trojan, shadowsocks, ...
	Tag            string // тег инбаунда
	Address        string // адрес сервера (IP или домен)
	Port           int
	CredentialJSON string // {"id":"..."} или {"password":"..."}
	SettingsJSON   string // settings инбаунда (decryption, method, ...)
	StreamJSON     string // streamSettings инбаунда (tls, reality, ws, ...)
	Remark         string // имя записи
}

// Generate возвращает ссылку для протокола.
// Если протокол не поддержан — возвращает ошибку.
func Generate(in Input) (string, error) {
	if in.Address == "" {
		return "", fmt.Errorf("address is required")
	}
	if in.Port < 1 || in.Port > 65535 {
		return "", fmt.Errorf("invalid port: %d", in.Port)
	}

	switch in.Protocol {
	case "vless":
		return generateVLESS(in)
	case "vmess":
		return generateVMess(in)
	case "trojan":
		return generateTrojan(in)
	case "shadowsocks":
		return generateShadowsocks(in)
	default:
		return "", fmt.Errorf("unsupported protocol: %s", in.Protocol)
	}
}

// parseJSON разбирает JSON-строку в map. Пустая строка и "{}" → пустой map.
func parseJSON(s string) (map[string]any, error) {
	out := map[string]any{}
	if s == "" || s == "{}" {
		return out, nil
	}
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		return nil, fmt.Errorf("invalid json: %w", err)
	}
	return out, nil
}

// getString достаёт строку из map.
func getString(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}
