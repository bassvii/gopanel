// SPDX-License-Identifier: AGPL-3.0-or-later

// Command gopanel — веб-панель управления прокси-сервером.
package main

import (
	"fmt"
	"os"

	"github.com/bassvii/gopanel/internal/config"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	// Пока единственная команда — run. Остальные (setup-token, reset-password,
	// config) появятся на следующих шагах Этапа 1.
	switch {
	case len(args) == 0 || args[0] == "run":
		var rest []string
		if len(args) > 0 {
			rest = args[1:]
		}
		return runServe(rest)
	default:
		return fmt.Errorf("unknown command: %s", args[0])
	}
}

func runServe(args []string) error {
	cfg, err := config.Load("", args)
	if err != nil {
		return err
	}
	fmt.Printf("config loaded: db=%s listen=%s:%d base_path=%q\n",
		cfg.DBPath, cfg.Listen, cfg.Port, cfg.BasePath)
	// Реальный запуск сервера — на шаге 1.3.
	return nil
}
