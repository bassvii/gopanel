// SPDX-License-Identifier: AGPL-3.0-or-later

// Command gopanel — веб-панель управления прокси-сервером.
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/bassvii/gopanel/internal/api"
	"github.com/bassvii/gopanel/internal/config"
	"github.com/bassvii/gopanel/internal/db"
)

const (
	settingAdminPort     = "admin_port"
	settingAdminBasePath = "admin_base_path"
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

	log := newLogger(cfg.LogLevel)

	conn, err := db.Open(cfg.DBPath)
	if err != nil {
		return err
	}
	defer conn.Close()

	port, err := resolvePort(conn, cfg.Port)
	if err != nil {
		return err
	}

	basePath, err := resolveBasePath(conn, cfg.BasePath)
	if err != nil {
		return err
	}

	srv, err := api.New(api.Config{
		Listen:   cfg.Listen,
		Port:     port,
		BasePath: basePath,
	}, conn, log)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() { errCh <- srv.Start() }()

	// Ждём, пока сервер выберет фактический порт, и сохраняем его.
	actualPort, err := waitForPort(ctx, srv, 5*time.Second)
	if err != nil {
		return err
	}
	if err := persistPort(conn, actualPort); err != nil {
		return err
	}

	select {
	case <-ctx.Done():
		log.Info("shutdown signal received")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Stop(shutdownCtx)
	case err := <-errCh:
		return err
	}
}

// resolvePort выбирает порт: приоритет — явно заданный (флаг/env/файл),
// иначе значение из БД, иначе 0 (ОС выберет свободный).
func resolvePort(conn *sql.DB, requested int) (int, error) {
	if requested != 0 {
		return requested, nil
	}
	v, ok, err := db.GetSetting(conn, settingAdminPort)
	if err != nil {
		return 0, err
	}
	if !ok {
		return 0, nil
	}
	p, err := strconv.Atoi(v)
	if err != nil || p < 1 || p > 65535 {
		return 0, nil
	}
	return p, nil
}

// resolveBasePath берёт base_path из явного значения, из БД,
// или генерирует новый и сохраняет его.
func resolveBasePath(conn *sql.DB, requested string) (string, error) {
	if requested != "" {
		return requested, nil
	}
	v, ok, err := db.GetSetting(conn, settingAdminBasePath)
	if err != nil {
		return "", err
	}
	if ok && v != "" {
		return v, nil
	}
	generated, err := generateBasePath()
	if err != nil {
		return "", err
	}
	if err := db.SetSetting(conn, settingAdminBasePath, generated); err != nil {
		return "", err
	}
	return generated, nil
}

// persistPort сохраняет фактический порт в БД. Если он не менялся —
// запись всё равно безвредна.
func persistPort(conn *sql.DB, port int) error {
	if port == 0 {
		return nil
	}
	return db.SetSetting(conn, settingAdminPort, strconv.Itoa(port))
}

// waitForPort ждёт, пока srv.Start выставит порт.
func waitForPort(ctx context.Context, srv *api.Server, timeout time.Duration) (int, error) {
	deadline := time.Now().Add(timeout)
	for {
		if p := srv.Port(); p != 0 {
			return p, nil
		}
		if time.Now().After(deadline) {
			return 0, fmt.Errorf("server did not bind a port within %s", timeout)
		}
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		case <-time.After(20 * time.Millisecond):
		}
	}
}

func generateBasePath() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "/" + hex.EncodeToString(b), nil
}

func newLogger(level string) *slog.Logger {
	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: lvl}))
}
