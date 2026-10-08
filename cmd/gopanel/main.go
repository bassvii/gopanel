// SPDX-License-Identifier: AGPL-3.0-or-later

// Command gopanel — веб-панель управления прокси-сервером.
package main

import (
	"fmt"
	"os"

	"github.com/bassvii/gopanel/internal/config"
	"github.com/bassvii/gopanel/internal/db"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
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

	conn, err := db.Open(cfg.DBPath)
	if err != nil {
		return err
	}
	defer conn.Close()

	fmt.Printf("config loaded: db=%s listen=%s:%d base_path=%q\n",
		cfg.DBPath, cfg.Listen, cfg.Port, cfg.BasePath)
	fmt.Println("database ready")
	// Реальный запуск сервера — на шаге 1.3.
	return nil
}
