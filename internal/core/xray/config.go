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

// BuildConfig собирает конфиг Xray из входных данных.
// Админский API (StatsService) добавляется автоматически:
// без него панель не сможет читать статистику.
func BuildConfig(in BuildInput) (*Config, error) {
	cfg := &Config{
		Log:       map[string]any{"loglevel": "warning"},
		Stats:     map[string]any{},
		Policy:    defaultPolicy(),
		API:       defaultAPI(),
		Outbounds: []map[string]any{{"protocol": "freedom", "tag": "direct"}},
		Routing:   defaultRouting(),
	}

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

	// Добавляем API-инбаунд для StatsService.
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

func defaultRouting() map[string]any {
	return map[string]any{
		"rules": []map[string]any{
			{
				"type":        "field",
				"inboundTag":  []string{"api"},
				"outboundTag": "api",
			},
		},
	}
}
