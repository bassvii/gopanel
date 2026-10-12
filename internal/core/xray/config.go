// SPDX-License-Identifier: AGPL-3.0-or-later

package xray

import (
	"encoding/json"
	"fmt"
)

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
}

// Inbound — инбаунд в формате Xray.
type Inbound struct {
	Tag            string         `json:"tag"`
	Listen         string         `json:"listen,omitempty"`
	Port           int            `json:"port"`
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
	Inbounds []InboundInput
	Outbounds []OutboundInput
	Rules     []RoutingRuleInput
}

// InboundInput — инбаунд и его пользователи.
type InboundInput struct {
	ID             int64
	Tag            string
	Protocol       string
	Port           int
	Listen         string
	SettingsJSON   string
	StreamJSON     string
	SniffingJSON   string
	Enabled        bool
	Users          []UserInput
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
// Админский API (StatsService) добавляется автоматически:
// без него панель не сможет читать статистику.
// BuildConfig собирает конфиг Xray из входных данных.
func BuildConfig(in BuildInput) (*Config, error) {
	cfg := &Config{
		Log:       map[string]any{"loglevel": "warning"},
		Stats:     map[string]any{},
		Policy:    defaultPolicy(),
		API:       defaultAPI(),
		Outbounds: buildOutbounds(in.Outbounds),
		Routing:   buildRouting(in.Rules),
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

		clients, err := buildClients(in.Protocol, in.Users)
		if err != nil {
			return nil, fmt.Errorf("inbound %q: %w", in.Tag, err)
		}
		if len(clients) > 0 {
			settings["clients"] = clients
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

	// API-инбаунд для StatsService.
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
// Структура зависит от протокола.
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
		"services": []string{"StatsService", "HandlerService"},
	}
}


// buildOutbounds формирует список outbound'ов для Xray.
// Всегда добавляет freedom и blackhole, если их нет.
func buildOutbounds(list []OutboundInput) []map[string]any {
	hasFreedom := false
	hasBlackhole := false
	for _, o := range list {
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
			continue
		}
		stream, err := parseSettings(o.StreamJSON)
		if err != nil {
			continue
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

	return out
}

// buildRouting формирует блок routing.
func buildRouting(rules []RoutingRuleInput) map[string]any {
	var out []map[string]any

	// Правило для API — всегда первое.
	out = append(out, map[string]any{
		"type":        "field",
		"inboundTag":  []string{"api"},
		"outboundTag": "api",
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

		// Домены.
		if domains, err := parseJSONArray(r.DomainsJSON); err == nil && len(domains) > 0 {
			rule["domain"] = domains
		}
		// IP.
		if ips, err := parseJSONArray(r.IPsJSON); err == nil && len(ips) > 0 {
			rule["ip"] = ips
		}
		// Порты.
		if ports, err := parseJSONArray(r.PortsJSON); err == nil && len(ports) > 0 {
			rule["port"] = ports
		}
		// Source IP.
		if src, err := parseJSONArray(r.SourceIPsJSON); err == nil && len(src) > 0 {
			rule["source"] = src
		}
		// Network.
		if r.Network != "" {
			rule["network"] = r.Network
		}
		// Inbound tags.
		if inbounds, err := parseJSONArray(r.InboundTagsJSON); err == nil && len(inbounds) > 0 {
			rule["inboundTag"] = inbounds
		}
		// Protocol.
		if proto, err := parseJSONArray(r.ProtocolJSON); err == nil && len(proto) > 0 {
			rule["protocol"] = proto
		}

		out = append(out, rule)
	}

	return map[string]any{"rules": out}
}

// parseJSONArray разбирает JSON-строку в массив строк.
func parseJSONArray(s string) ([]string, error) {
	if s == "" || s == "[]" {
		return nil, nil
	}
	var out []string
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		return nil, err
	}
	return out, nil
}
