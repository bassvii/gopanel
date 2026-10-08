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
)

// Config — все настройки панели.
type Config struct {
	DBPath   string `json:"db_path"`
	Listen   string `json:"listen"`
	Port     int    `json:"port"`
	BasePath string `json:"base_path"`
	XrayBin  string `json:"xray_bin"`
	LogLevel string `json:"log_level"`
	XrayConfigPath string `json:"xray_config_path"`
}

// Default возвращает конфиг со значениями по умолчанию.
func Default() Config {
	return Config{
		DBPath:   "./gopanel.db",
		Listen:   "127.0.0.1",
		Port:     0,
		BasePath: "",
		XrayBin:  "/usr/local/bin/xray",
		LogLevel: "info",
		XrayConfigPath: "/tmp/gopanel-xray.json",
	}
}

// Load собирает конфиг из всех источников.
// path — путь к JSON-файлу конфигурации, может быть пустым.
// args — аргументы командной строки без имени программы.
func Load(path string, args []string) (Config, error) {
	cfg := Default()

	// 1. Файл конфигурации.
	if path != "" {
		if err := loadFile(path, &cfg); err != nil {
			return cfg, fmt.Errorf("config file: %w", err)
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
}

func applyFlags(cfg *Config, args []string) error {
	fs := flag.NewFlagSet("gopanel", flag.ContinueOnError)

	fs.StringVar(&cfg.DBPath, "db", cfg.DBPath, "путь к файлу SQLite")
	fs.IntVar(&cfg.Port, "port", cfg.Port, "порт админского listener (0 — случайный)")
	fs.StringVar(&cfg.BasePath, "base-path", cfg.BasePath, "секретный базовый путь")
	fs.StringVar(&cfg.XrayBin, "xray-bin", cfg.XrayBin, "путь к бинарнику Xray")
	fs.StringVar(&cfg.LogLevel, "log-level", cfg.LogLevel, "уровень логов: debug, info, warn, error")
	fs.StringVar(&cfg.XrayConfigPath, "xray-config", cfg.XrayConfigPath, "путь к файлу конфига Xray")

	if err := fs.Parse(args); err != nil {
		return err
	}
	if cfg.Port < 0 || cfg.Port > 65535 {
        return errors.New("port must be 0..65535")
	}
	return nil
}
