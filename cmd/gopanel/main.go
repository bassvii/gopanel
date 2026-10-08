// SPDX-License-Identifier: AGPL-3.0-or-later

// Command gopanel — веб-панель управления прокси-сервером.
package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/bassvii/gopanel/internal/api"
	"github.com/bassvii/gopanel/internal/auth"
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
	if len(args) == 0 {
		return runServe(nil)
	}

	switch args[0] {
	case "run":
		return runServe(args[1:])
	case "admin":
		return runAdmin(args[1:])
	default:
		return fmt.Errorf("unknown command: %s", args[0])
	}
}

func runAdmin(args []string) error {
	if len(args) == 0 || args[0] != "create" {
		return fmt.Errorf("usage: gopanel admin create --user NAME [--password PASS]")
	}

	fs := flag.NewFlagSet("admin create", flag.ContinueOnError)
	var (
		user     = fs.String("user", "", "имя администратора")
		password = fs.String("password", "", "пароль (если пусто — из env GOPANEL_ADMIN_PASSWORD)")
		dbPath   = fs.String("db", "", "путь к БД (если пусто — из конфига)")
	)
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}

	if *user == "" {
		return fmt.Errorf("--user is required")
	}
	if *password == "" {
		*password = os.Getenv("GOPANEL_ADMIN_PASSWORD")
	}
	if *password == "" {
		return fmt.Errorf("password is required (--password or GOPANEL_ADMIN_PASSWORD)")
	}

	cfg, err := config.Load("", nil)
	if err != nil {
		return err
	}
	if *dbPath != "" {
		cfg.DBPath = *dbPath
	}

	conn, err := db.Open(cfg.DBPath)
	if err != nil {
		return err
	}
	defer conn.Close()

	id, err := auth.CreateAdmin(conn, *user, *password)
	if err != nil {
		return err
	}
	fmt.Printf("admin %q created (id=%d)\n", *user, id)
	return nil
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

	// Если админов ещё нет — попробовать создать из env (для Docker).
	if err := ensureFirstAdmin(conn, log); err != nil {
		return err
	}

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

// ensureFirstAdmin создаёт первого админа из env-переменных,
// если админов ещё нет. Для Docker.
func ensureFirstAdmin(conn *sql.DB, log *slog.Logger) error {
	n, err := auth.CountAdmins(conn)
	if err != nil {
		return err
	}
	if n > 0 {
		return nil
	}

	user := os.Getenv("GOPANEL_ADMIN_USER")
	pass := os.Getenv("GOPANEL_ADMIN_PASSWORD")
	if user == "" || pass == "" {
		return nil
	}

	id, err := auth.CreateAdmin(conn, user, pass)
	if err != nil {
		return fmt.Errorf("create first admin from env: %w", err)
	}
	log.Info("first admin created from env", "username", user, "id", id)
	return nil
}

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

func persistPort(conn *sql.DB, port int) error {
	if port == 0 {
		return nil
	}
	return db.SetSetting(conn, settingAdminPort, strconv.Itoa(port))
}

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
