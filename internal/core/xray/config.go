// SPDX-License-Identifier: AGPL-3.0-or-later

package xray

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// APIAddr — адрес gRPC StatsService Xray (loopback TCP).
//
// Unix-сокет не используется: Xray 26.3.27 падает с паникой
// (strconv.Atoi: parsing "") при первом соединении через сокет.
const APIAddr = "127.0.0.1:10085"

// APISocketPath — устаревшая функция (Unix-сокет не используется).
// Оставлена, чтобы не ломать возможные внешние вызовы.
//
// Deprecated: используйте APIAddr.
func APISocketPath() string {
	for _, dir := range []string{"/run/gopanel", filepath.Join(os.TempDir(), "gopanel")} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			continue
		}
		probe := filepath.Join(dir, ".w")
		f, err := os.OpenFile(probe, os.O_CREATE|os.O_WRONLY, 0o600)
		if err != nil {
			continue
		}
		_ = f.Close()
		_ = os.Remove(probe)
		return filepath.Join(dir, "xray-api.sock")
	}
	return filepath.Join(os.TempDir(), "xray-api.sock")
}

// RemoveStaleAPISocket — устаревшая функция, удаляет остатки Unix-сокета.
//
// Deprecated: Unix-сокет не используется.
func RemoveStaleAPISocket() {
	sock := APISocketPath()
	_ = os.Remove(sock)
	_ = os.Remove(sock + ".lock")
}

// Config — минимальная структура Xray-конфига, которую мы собираем.
// Остальные поля (log, dns, routing, policy, stats, api) добавляются
// отдельно, чтобы не терять пользовательские настройки.
type Config struct {
	Log       map[string]any   `json:"log,omitempty"`
	API       map[string]any   `json:"api,omitempty"`
	Stats     map[string]any   `json:"stats"`
	Policy    map[string]any   `json:"policy,omitempty"`
	Inbounds  []Inbound        `json:"inbounds"`
	Outbounds []map[string]any `json:"outbounds"`
	Routing   map[string]any   `json:"routing,omitempty"`
	Skipped   []string         `json:"-"`
}

// Inbound — инбаунд в формате Xray.
type Inbound struct {
	Tag            string         `json:"tag"`
	Listen         string         `json:"listen,omitempty"`
	Port           int            `json:"port,omitempty"`
	Protocol       string         `json:"protocol"`
	Settings       map[string]any `json:"settings,omitempty"`
	StreamSettings map[string]any `json:"streamSettings,omitempty"`
	Sniffing       map[string]any `json:"sniffing,omitempty"`
}

// Client — клиент (пользователь) внутри инбаунда.
type Client struct {
	ID       string `json:"id,omitempty"`
	Email    string `json:"email"`
	Password string `json:"password,omitempty"`
	Method   string `json:"method,omitempty"`
	Flow     string `json:"flow,omitempty"`
	Level    int    `json:"level,omitempty"`
}

// BuildInput — данные из БД, нужные для сборки конфига.
type BuildInput struct {
	Inbounds  []InboundInput
	Outbounds []OutboundInput
	Rules     []RoutingRuleInput
}

// InboundInput — инбаунд и его пользователи.
type InboundInput struct {
	ID           int64
	Tag          string
	Protocol     string
	Port         int
	Listen       string
	SettingsJSON string
	StreamJSON   string
	SniffingJSON string
	Enabled      bool
	Users        []UserInput
}

// UserInput — пользователь и его учётные данные для инбаунда.
type UserInput struct {
	UserID         int64
	Name           string
	Email          string
	Enabled        bool
	CredentialJSON string
}

// OutboundInput — outbound из БД.
type OutboundInput struct {
	Tag          string
	Protocol     string
	SettingsJSON string
	StreamJSON   string
	Enabled      bool
}

// RoutingRuleInput — правило маршрутизации из БД.
type RoutingRuleInput struct {
	ID              int64
	Position        int
	RuleTag         string
	DomainsJSON     string
	IPsJSON         string
	PortsJSON       string
	SourceIPsJSON   string
	Network         string
	InboundTagsJSON string
	ProtocolJSON    string
	OutboundTag     string
	Enabled         bool
}

// BuildConfig собирает конфиг Xray из входных данных.
func BuildConfig(in BuildInput) (*Config, error) {
	outbounds, err := buildOutbounds(in.Outbounds)
	if err != nil {
		return nil, fmt.Errorf("outbounds: %w", err)
	}

	routing, err := buildRouting(in.Rules)
	if err != nil {
		return nil, fmt.Errorf("routing: %w", err)
	}

	cfg := &Config{
		Log:       map[string]any{"loglevel": "warning"},
		Stats:     map[string]any{},
		Policy:    defaultPolicy(),
		API:       defaultAPI(),
		Outbounds: outbounds,
		Routing:   routing,
	}

	// Инбаунды.
	for _, in := range in.Inbounds {
		if !in.Enabled {
			continue
		}

		settings, err := parseSettings(in.SettingsJSON)
		if err != nil {
			return nil, fmt.Errorf("inbound %q settings: %w", in.Tag, err)
		}
		stream, err := parseSettings(in.StreamJSON)
		if err != nil {
			return nil, fmt.Errorf("inbound %q stream: %w", in.Tag, err)
		}
		sniffing, err := parseSettings(in.SniffingJSON)
		if err != nil {
			return nil, fmt.Errorf("inbound %q sniffing: %w", in.Tag, err)
		}

		switch in.Protocol {
		case "socks", "http":
			accts, err := buildSOCKSAccounts(in.Users, settings)
			if err != nil {
				return nil, fmt.Errorf("inbound %q: %w", in.Tag, err)
			}
			if len(accts) == 0 {
				cfg.Skipped = append(cfg.Skipped,
					fmt.Sprintf("inbound %q (%s): нет аккаунтов — пропущен (открытый прокси запрещён)", in.Tag, in.Protocol))
				continue
			}
			settings["accounts"] = accts
			if in.Protocol == "socks" {
				settings["auth"] = "password"
			}
			delete(settings, "users")

		case "hysteria":
			hyUsers, err := buildHysteriaUsers(in.Users)
			if err != nil {
				return nil, fmt.Errorf("inbound %q: %w", in.Tag, err)
			}
			if len(hyUsers) > 0 {
				settings["users"] = hyUsers
			}

		default:
			clients, err := buildClients(in.Protocol, in.Users)
			if err != nil {
				return nil, fmt.Errorf("inbound %q: %w", in.Tag, err)
			}
			if len(clients) > 0 {
				settings["clients"] = clients
			}
		}

		cfg.Inbounds = append(cfg.Inbounds, Inbound{
			Tag:            in.Tag,
			Listen:         in.Listen,
			Port:           in.Port,
			Protocol:       in.Protocol,
			Settings:       settings,
			StreamSettings: stream,
			Sniffing:       sniffing,
		})
	}

	// API-инбаунд для StatsService: loopback TCP, порт 10085.
	// Порт зарезервирован (см. validateInbound в internal/api/inbounds.go).
	cfg.Inbounds = append(cfg.Inbounds, Inbound{
		Tag:      "api",
		Listen:   "127.0.0.1",
		Port:     10085,
		Protocol: "dokodemo-door",
		Settings: map[string]any{"address": "127.0.0.1"},
	})

	return cfg, nil
}

// MarshalConfig сериализует конфиг в JSON с отступами.
func MarshalConfig(cfg *Config) ([]byte, error) {
	return json.MarshalIndent(cfg, "", "  ")
}

// buildClients собирает список клиентов для инбаунда.
func buildClients(protocol string, users []UserInput) ([]Client, error) {
	var clients []Client
	for _, u := range users {
		if !u.Enabled {
			continue
		}
		cred := map[string]any{}
		if u.CredentialJSON != "" && u.CredentialJSON != "{}" {
			if err := json.Unmarshal([]byte(u.CredentialJSON), &cred); err != nil {
				return nil, fmt.Errorf("user %q credential: %w", u.Name, err)
			}
		}

		c := Client{Email: u.Email}
		switch protocol {
		case "vless", "vmess":
			if id, ok := cred["id"].(string); ok {
				c.ID = id
			}
			if flow, ok := cred["flow"].(string); ok {
				c.Flow = flow
			}
		case "trojan", "shadowsocks":
			if pw, ok := cred["password"].(string); ok {
				c.Password = pw
			}
			if m, ok := cred["method"].(string); ok {
				c.Method = m
			}
		case "socks", "http":
			if user, ok := cred["user"].(string); ok {
				c.ID = user
			}
			if pw, ok := cred["password"].(string); ok {
				c.Password = pw
			}
		}

		clients = append(clients, c)
	}
	return clients, nil
}

func parseSettings(s string) (map[string]any, error) {
	out := map[string]any{}
	if s == "" || s == "{}" {
		return out, nil
	}
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		return nil, fmt.Errorf("invalid json: %w", err)
	}
	return out, nil
}

func defaultPolicy() map[string]any {
	return map[string]any{
		"levels": map[string]any{
			"0": map[string]any{
				"statsUserUplink":   true,
				"statsUserDownlink": true,
				"statsUserOnline":   true,
			},
		},
		"system": map[string]any{
			"statsInboundUplink":   true,
			"statsInboundDownlink": true,
		},
	}
}

func defaultAPI() map[string]any {
	return map[string]any{
		"tag":      "api",
		"services": []string{"StatsService"},
	}
}

// buildOutbounds формирует список outbound'ов для Xray.
func buildOutbounds(list []OutboundInput) ([]map[string]any, error) {
	hasFreedom := false
	hasBlackhole := false
	for _, o := range list {
		if !o.Enabled {
			continue
		}
		switch o.Protocol {
		case "freedom":
			hasFreedom = true
		case "blackhole":
			hasBlackhole = true
		}
	}

	var out []map[string]any

	// Системные.
	if !hasFreedom {
		out = append(out, map[string]any{
			"protocol": "freedom",
			"tag":      "DIRECT",
		})
	}
	if !hasBlackhole {
		out = append(out, map[string]any{
			"protocol": "blackhole",
			"tag":      "BLOCK",
		})
	}

	// Пользовательские.
	for _, o := range list {
		if !o.Enabled {
			continue
		}
		settings, err := parseSettings(o.SettingsJSON)
		if err != nil {
			return nil, fmt.Errorf("outbound %q settings: %w", o.Tag, err)
		}
		stream, err := parseSettings(o.StreamJSON)
		if err != nil {
			return nil, fmt.Errorf("outbound %q stream: %w", o.Tag, err)
		}

		o2 := map[string]any{
			"protocol": o.Protocol,
			"tag":      o.Tag,
		}
		if len(settings) > 0 {
			o2["settings"] = settings
		}
		if len(stream) > 0 {
			o2["streamSettings"] = stream
		}
		out = append(out, o2)
	}

	return out, nil
}

// buildRouting формирует блок routing.
func buildRouting(rules []RoutingRuleInput) (map[string]any, error) {
	var out []map[string]any

	// Правило для API — всегда первое.
	out = append(out, map[string]any{
		"type":        "field",
		"inboundTag":  []string{"api"},
		"outboundTag": "api",
	})

	// Блокируем приватные адреса и служебные домены.
	out = append(out, map[string]any{
		"type": "field",
		"domain": []string{
			"localhost",
			"metadata.google.internal",
			"metadata.google.com",
		},
		"outboundTag": "BLOCK",
		"ruleTag":     "Служебные домены — BLOCK (защита)",
	})

	out = append(out, map[string]any{
		"type":        "field",
		"ip":          []string{"geoip:private"},
		"outboundTag": "BLOCK",
		"ruleTag":     "Приватные адреса — BLOCK (защита)",
	})

	// Правила из БД — по порядку.
	for _, r := range rules {
		if !r.Enabled {
			continue
		}
		rule := map[string]any{
			"type":        "field",
			"outboundTag": r.OutboundTag,
		}
		if r.RuleTag != "" {
			rule["ruleTag"] = r.RuleTag
		}

		cond := 0
		add := func(key, js string) error {
			arr, err := ParseJSONArray(js)
			if err != nil {
				return fmt.Errorf("rule %q, поле %s: %w", r.RuleTag, key, err)
			}
			if len(arr) > 0 {
				rule[key] = arr
				cond++
			}
			return nil
		}
		if err := add("domain", r.DomainsJSON); err != nil {
			return nil, err
		}
		if err := add("ip", r.IPsJSON); err != nil {
			return nil, err
		}
		if err := add("port", r.PortsJSON); err != nil {
			return nil, err
		}
		if err := add("source", r.SourceIPsJSON); err != nil {
			return nil, err
		}
		if err := add("inboundTag", r.InboundTagsJSON); err != nil {
			return nil, err
		}
		if err := add("protocol", r.ProtocolJSON); err != nil {
			return nil, err
		}
		if r.Network != "" {
			rule["network"] = r.Network
			cond++
		}
		if cond == 0 {
			return nil, fmt.Errorf("rule %q: нет ни одного условия (правило поймало бы весь трафик)", r.RuleTag)
		}
		out = append(out, rule)
	}

	return map[string]any{
		"domainStrategy": "IPOnDemand",
		"rules":          out,
	}, nil
}

// ParseJSONArray разбирает JSON-строку в массив строк.
func ParseJSONArray(s string) ([]string, error) {
	if s == "" || s == "[]" {
		return nil, nil
	}
	var raw []any
	if err := json.Unmarshal([]byte(s), &raw); err != nil {
		return nil, fmt.Errorf("ожидается JSON-массив: %w", err)
	}
	out := make([]string, 0, len(raw))
	for _, v := range raw {
		switch x := v.(type) {
		case string:
			if strings.TrimSpace(x) == "" {
				return nil, errors.New("пустая строка в массиве")
			}
			out = append(out, x)
		case float64:
			out = append(out, strconv.FormatFloat(x, 'f', -1, 64))
		default:
			return nil, fmt.Errorf("недопустимый элемент %v (нужна строка или число)", v)
		}
	}
	return out, nil
}

// buildSOCKSAccounts собирает settings.accounts для SOCKS/HTTP.
func buildSOCKSAccounts(users []UserInput, settings map[string]any) ([]map[string]string, error) {
	out := make([]map[string]string, 0, len(users))
	for _, u := range users {
		if !u.Enabled {
			continue
		}
		cred := map[string]any{}
		if u.CredentialJSON != "" && u.CredentialJSON != "{}" {
			if err := json.Unmarshal([]byte(u.CredentialJSON), &cred); err != nil {
				return nil, fmt.Errorf("user %q credential: %w", u.Name, err)
			}
		}
		user := getStringFromMap(cred, "user")
		pass := getStringFromMap(cred, "pass")
		if pass == "" {
			pass = getStringFromMap(cred, "password")
		}
		if user == "" || pass == "" {
			continue
		}
		out = append(out, map[string]string{"user": user, "pass": pass})
	}

	if len(out) == 0 {
		if raw, ok := settings["users"].([]any); ok {
			for _, item := range raw {
				if m, ok := item.(map[string]any); ok {
					user := getStringFromMap(m, "user")
					pass := getStringFromMap(m, "pass")
					if pass == "" {
						pass = getStringFromMap(m, "password")
					}
					if user != "" && pass != "" {
						out = append(out, map[string]string{"user": user, "pass": pass})
					}
				}
			}
		}
	}
	return out, nil
}

// buildHysteriaUsers собирает settings.users[{auth, email}] для Hysteria2.
func buildHysteriaUsers(users []UserInput) ([]map[string]any, error) {
	out := make([]map[string]any, 0, len(users))
	for _, u := range users {
		if !u.Enabled {
			continue
		}
		cred := map[string]any{}
		if u.CredentialJSON != "" && u.CredentialJSON != "{}" {
			if err := json.Unmarshal([]byte(u.CredentialJSON), &cred); err != nil {
				return nil, fmt.Errorf("user %q credential: %w", u.Name, err)
			}
		}
		auth := getStringFromMap(cred, "auth")
		if auth == "" {
			auth = getStringFromMap(cred, "password")
		}
		if auth == "" {
			continue
		}
		out = append(out, map[string]any{
			"auth":  auth,
			"email": u.Email,
		})
	}
	return out, nil
}

// getStringFromMap — утилита, извлекает строку из map[string]any.
func getStringFromMap(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}
