// SPDX-License-Identifier: AGPL-3.0-or-later

// Package config собирает настройки панели из файла, переменных
// окружения и флагов командной строки. Приоритет: флаги > окружение > файл > значения по умолчанию.
package config

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	DBPath           string `json:"db_path"`
	Listen           string `json:"listen"`
	Port             int    `json:"port"`
	BasePath         string `json:"base_path"`
	XrayBin          string `json:"xray_bin"`
	LogLevel         string `json:"log_level"`
	XrayConfigPath   string `json:"xray_config_path"`
	AllowNonLoopback bool   `json:"allow_non_loopback"`
	// SecureCookies выставляет флаг Secure на cookie сессии.
	// Включать, когда панель за HTTPS-прокси (иначе cookie уйдёт
	// и по HTTP). Для loopback по умолчанию false.
	SecureCookies bool `json:"secure_cookies"`

	// TrustedProxies — список CIDR, от которых принимается
	// X-Forwarded-For. Если RemoteAddr не входит в этот список,
	// заголовок игнорируется (защита от подмены IP).
	// Пример: ["127.0.0.1/32", "::1/128", "10.0.0.0/8"].
	TrustedProxies []string `json:"trusted_proxies"`
}

// Default возвращает конфиг со значениями по умолчанию.
func Default() Config {
	return Config{
		DBPath:           "./gopanel.db",
		Listen:           "127.0.0.1",
		Port:             0,
		BasePath:         "",
		XrayBin:          "/usr/local/bin/xray",
		LogLevel:         "info",
		XrayConfigPath:   "/tmp/gopanel-xray.json",
		AllowNonLoopback: false,
	}
}

// Load собирает конфиг из всех источников.
// path — путь к JSON-файлу конфигурации, может быть пустым.
// args — аргументы командной строки без имени программы.
func Load(path string, args []string) (Config, error) {
	cfg := Default()

	// Разбираем --config из args до всего остального: этот флаг
	// указывает, откуда читать файл, и должен быть известен заранее.
	// Приоритет: --config > env GOPANEL_CONFIG > path из вызова.
	fileFromFlags, err := extractConfigFlag(args)
	if err != nil {
		return cfg, err
	}
	if fileFromFlags != "" {
		path = fileFromFlags
	} else if path == "" {
		path = os.Getenv("GOPANEL_CONFIG")
	}

	// 1. Файл конфигурации (если путь известен).
	if path != "" {
		if err := loadFile(path, &cfg); err != nil {
			return cfg, fmt.Errorf("config file %s: %w", path, err)
		}
	}

	// 2. Переменные окружения.
	applyEnv(&cfg)

	// 3. Флаги командной строки.
	if err := applyFlags(&cfg, args); err != nil {
		return cfg, fmt.Errorf("flags: %w", err)
	}

	return cfg, nil
}

// extractConfigFlag вытаскивает значение --config / -config из args,
// не мешая остальным флагам. Возвращает "" если флага нет.
// Поддерживает формы: --config PATH, --config=PATH, -config PATH.
func extractConfigFlag(args []string) (string, error) {
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--config" || a == "-config" {
			if i+1 >= len(args) {
				return "", fmt.Errorf("--config requires a value")
			}
			return args[i+1], nil
		}
		if strings.HasPrefix(a, "--config=") {
			return strings.TrimPrefix(a, "--config="), nil
		}
		if strings.HasPrefix(a, "-config=") {
			return strings.TrimPrefix(a, "-config="), nil
		}
	}
	return "", nil
}

func loadFile(path string, cfg *Config) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, cfg); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	return nil
}

func applyEnv(cfg *Config) {
	if v := os.Getenv("GOPANEL_DB_PATH"); v != "" {
		cfg.DBPath = v
	}
	if v := os.Getenv("GOPANEL_LISTEN"); v != "" {
		cfg.Listen = v
	}
	if v := os.Getenv("GOPANEL_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.Port = p
		}
	}
	if v := os.Getenv("GOPANEL_BASE_PATH"); v != "" {
		cfg.BasePath = v
	}
	if v := os.Getenv("GOPANEL_XRAY_BIN"); v != "" {
		cfg.XrayBin = v
	}
	if v := os.Getenv("GOPANEL_LOG_LEVEL"); v != "" {
		cfg.LogLevel = v
	}
	if v := os.Getenv("GOPANEL_XRAY_CONFIG"); v != "" {
		cfg.XrayConfigPath = v
	}
	if os.Getenv("GOPANEL_ALLOW_NON_LOOPBACK") == "1" {
		cfg.AllowNonLoopback = true
	}
	if os.Getenv("GOPANEL_SECURE_COOKIES") == "1" {
		cfg.SecureCookies = true
	}
	if v := os.Getenv("GOPANEL_TRUSTED_PROXIES"); v != "" {
		cfg.TrustedProxies = splitCSV(v)
	}
}

// splitCSV разбивает строку по запятой, тримит пробелы, отбрасывает
// пустые элементы. Нужен для env-переменных со списком.
func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func applyFlags(cfg *Config, args []string) error {
	fs := flag.NewFlagSet("gopanel", flag.ContinueOnError)

	fs.StringVar(&cfg.DBPath, "db", cfg.DBPath, "путь к файлу SQLite")
	fs.StringVar(&cfg.Listen, "listen", cfg.Listen, "адрес для админского listener (по умолчанию 127.0.0.1)")
	fs.IntVar(&cfg.Port, "port", cfg.Port, "порт админского listener (0 — случайный)")
	fs.StringVar(&cfg.BasePath, "base-path", cfg.BasePath, "секретный базовый путь")
	fs.StringVar(&cfg.XrayBin, "xray-bin", cfg.XrayBin, "путь к бинарнику Xray")
	fs.StringVar(&cfg.LogLevel, "log-level", cfg.LogLevel, "уровень логов: debug, info, warn, error")
	fs.StringVar(&cfg.XrayConfigPath, "xray-config", cfg.XrayConfigPath, "путь к файлу конфига Xray")
	fs.BoolVar(&cfg.SecureCookies, "secure-cookies", cfg.SecureCookies, "выставлять Secure на cookie сессии (для HTTPS)")

	var trustedProxiesFlag string
	fs.StringVar(&trustedProxiesFlag, "trusted-proxies", "", "CIDR через запятую, от которых доверяем X-Forwarded-For")

	// --config уже разобран выше (extractConfigFlag), регистрируем его
	// как «принимаем, но игнорируем» — иначе flag.Parse упадёт.
	var _unused string
	fs.StringVar(&_unused, "config", "", "путь к JSON-конфигу (обрабатывается отдельно)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	// Флаг --trusted-proxies: если задан, переопределяет env.
	// Пустая строка = не трогать (значит, остаётся из env или файла).
	if trustedProxiesFlag != "" {
		cfg.TrustedProxies = splitCSV(trustedProxiesFlag)
	}
	if cfg.Port < 0 || cfg.Port > 65535 {
		return errors.New("port must be 0..65535")
	}
	return nil
}
