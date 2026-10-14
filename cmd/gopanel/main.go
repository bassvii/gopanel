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
	"path/filepath"
    "golang.org/x/term"

	"github.com/bassvii/gopanel/internal/system"
	"github.com/bassvii/gopanel/internal/backup"
	"github.com/bassvii/gopanel"
	"github.com/bassvii/gopanel/internal/api"
	"github.com/bassvii/gopanel/internal/audit"
	"github.com/bassvii/gopanel/internal/auth"
	"github.com/bassvii/gopanel/internal/config"
	"github.com/bassvii/gopanel/internal/core/xray"
	"github.com/bassvii/gopanel/internal/db"
	"github.com/bassvii/gopanel/internal/stats"
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
	case "reset-password":
		return runResetPassword(args[1:])
	case "config":
		return runConfig(args[1:])
	case "backup":
		return runBackup(args[1:])
	default:
		return fmt.Errorf("unknown command: %s", args[0])
	}
}

// --- admin ---

func runAdmin(args []string) error {
	if len(args) == 0 || args[0] != "create" {
		return fmt.Errorf("usage: gopanel admin create --user NAME [--password PASS]")
	}

	fs := flag.NewFlagSet("admin create", flag.ContinueOnError)
	var (
		user     = fs.String("user", "", "имя администратора")
		password = fs.String("password", "", "пароль (если пусто — из env GOPANEL_ADMIN_PASSWORD или интерактивно)")
		dbPath   = fs.String("db", "", "путь к БД (если пусто — из конфига)")
		cfgPath  = fs.String("config", "", "путь к JSON-конфигу (если пусто — из env GOPANEL_CONFIG)")
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
		// Спрашиваем пароль интерактивно, чтобы он не попадал
		// в ps и shell history.
		p, err := readPasswordInteractive(*user)
		if err != nil {
			return err
		}
		*password = p
	}

	cfg, err := config.Load(*cfgPath, nil)
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

// --- reset-password ---

func runResetPassword(args []string) error {
	fs := flag.NewFlagSet("reset-password", flag.ContinueOnError)
	var (
		user     = fs.String("user", "", "имя администратора")
		password = fs.String("password", "", "новый пароль (если пусто — из env GOPANEL_ADMIN_PASSWORD или интерактивно)")
		dbPath   = fs.String("db", "", "путь к БД (если пусто — из конфига)")
		cfgPath  = fs.String("config", "", "путь к JSON-конфигу (если пусто — из env GOPANEL_CONFIG)")
	)
	if err := fs.Parse(args); err != nil {
		return err
	}

	if *user == "" {
		return fmt.Errorf("--user is required")
	}
	if *password == "" {
		*password = os.Getenv("GOPANEL_ADMIN_PASSWORD")
	}
	if *password == "" {
		p, err := readPasswordInteractive(*user)
		if err != nil {
			return err
		}
		*password = p
	}

	cfg, err := config.Load(*cfgPath, nil)
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

	if err := auth.ResetPassword(conn, *user, *password); err != nil {
		return err
	}
	_ = audit.Log(conn, 0, audit.ActionPasswordReset, *user, "cli")

	fmt.Printf("password reset for %q; 2FA disabled; all sessions terminated\n", *user)
	return nil
}

// --- config ---

func runConfig(args []string) error {
	fs := flag.NewFlagSet("config", flag.ContinueOnError)
	var (
		port     = fs.Int("port", 0, "новый порт админки (0 — не менять)")
		basePath = fs.String("base-path", "", "новый base path (пусто — не менять)")
		dbPath   = fs.String("db", "", "путь к БД (если пусто — из конфига)")
		cfgPath  = fs.String("config", "", "путь к JSON-конфигу (если пусто — из env GOPANEL_CONFIG)")
	)
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, err := config.Load(*cfgPath, nil)
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

	if *port == 0 && *basePath == "" {
		p, _, _ := db.GetSetting(conn, settingAdminPort)
		b, _, _ := db.GetSetting(conn, settingAdminBasePath)
		fmt.Printf("port:      %s\nbase_path: %s\ndb_path:   %s\n", p, b, cfg.DBPath)
		return nil
	}

	if *port != 0 {
		if *port < 1 || *port > 65535 {
			return fmt.Errorf("port must be 1..65535")
		}
		if err := db.SetSetting(conn, settingAdminPort, strconv.Itoa(*port)); err != nil {
			return err
		}
		fmt.Printf("port set to %d\n", *port)
	}

	if *basePath != "" {
		if (*basePath)[0] != '/' {
			*basePath = "/" + *basePath
		}
		if err := db.SetSetting(conn, settingAdminBasePath, *basePath); err != nil {
			return err
		}
		fmt.Printf("base_path set to %s\n", *basePath)
	}
	fmt.Println("restart the panel to apply changes")
	return nil
}

// --- run ---

func runServe(args []string) error {
	// 1. Конфиг и логгер.
	cfg, err := config.Load("", args)
	if err != nil {
		return err
	}
	log := newLogger(cfg.LogLevel)

	// 2. База данных.
	conn, err := db.Open(cfg.DBPath)
	if err != nil {
		return err
	}
	defer conn.Close()

	// 3. Контекст отмены — ДО всего, что его использует.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// 4. Проверка, что админ вообще есть. Если нет — панель стартует,
	// но пишет в лог, что нужно создать админа через CLI.
	// Автосоздание из env убрано намеренно (см. SECURITY, пункт 2):
	// пароль не должен лежать в переменных окружения из compose.
	if err := checkAdminExists(conn, log); err != nil {
		return err
	}

	// 5. Порт и base path.
	port, err := resolvePort(conn, cfg.Port)
	if err != nil {
		return err
	}
	basePath, err := resolveBasePath(conn, cfg.BasePath)
	if err != nil {
		return err
	}
	if err := writeBasePathFile(cfg.DBPath, basePath); err != nil {
		return fmt.Errorf("write base path file: %w", err)
	}

	// 6. Xray.
	core, err := xray.New(cfg.XrayBin, cfg.XrayConfigPath, log)
	if err != nil {
		return fmt.Errorf("init xray core: %w", err)
	}

	configData, err := buildConfigFromDB(conn)
	if err != nil {
		return fmt.Errorf("build config: %w", err)
	}
	if err := core.ApplyConfig(ctx, configData); err != nil {
		log.Error("xray start failed", "err", err)
	}

	// 6.5. Системный сборщик метрик.
	sysCollector := system.NewCollector(
		5*time.Second,
		"/",
		720,
		log,
	)
	go sysCollector.Run(ctx)

	// 7. Админский HTTP-сервер.
	srv, err := api.New(api.Config{
		Listen:           cfg.Listen,
		Port:             port,
		BasePath:         basePath,
		AllowNonLoopback: cfg.AllowNonLoopback,
		SecureCookies:    cfg.SecureCookies,
		TrustedProxies:   cfg.TrustedProxies,
	}, conn, log)
	if err != nil {
		return err
	}

	// Устанавливаем ядро и системный сборщик.
	srv.SetCore(core)
	if v, err := core.Version(); err == nil {
		srv.SetXrayVersion(v)
	}
	srv.SetSystemCollector(sysCollector, cfg.DBPath, time.Now())

	// 8. Вшитый фронтенд.
	if webFS := gopanel.WebFS(); webFS != nil {
		srv.SetWebFS(webFS)
		log.Info("frontend embedded")
	}

	// 9. Сборщик статистики Xray.
	collector := stats.New(
		conn,
		cfg.XrayBin,
		"127.0.0.1:10085",
		30*time.Second,
		core,
		log,
	)
	go collector.Run(ctx)

	// 10. Цикл перезагрузки.
	go reloadLoop(ctx, srv, core, conn, log)

	// 11. Лимиты.
	go stats.RunLimitEnforcer(ctx, conn, srv.ReloadSignal(), 30*time.Second, log)

	// 12. HTTP-сервер.
	errCh := make(chan error, 1)
	go func() { errCh <- srv.Start() }()

	// 13. Порт.
	actualPort, err := waitForPort(ctx, srv, 5*time.Second)
	if err != nil {
		return err
	}
	if err := persistPort(conn, actualPort); err != nil {
		return err
	}

	// 14. Ожидание.
	select {
	case <-ctx.Done():
		log.Info("shutdown signal received")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := core.Stop(); err != nil {
			log.Warn("xray stop failed", "err", err)
		}
		return srv.Stop(shutdownCtx)
	case err := <-errCh:
		return err
	}
}
// reloadLoop слушает сигналы перезагрузки и перегенерирует конфиг Xray.
func reloadLoop(ctx context.Context, srv *api.Server, core *xray.Core, conn *sql.DB, log *slog.Logger) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-srv.ReloadCh():
			data, err := buildConfigFromDB(conn)
			if err != nil {
				log.Error("rebuild config failed", "err", err)
				continue
			}
			if err := core.ApplyConfig(ctx, data); err != nil {
				log.Error("apply config failed", "err", err)
				continue
			}
			log.Info("xray config reloaded")
		}
	}
}

// --- helpers ---

// checkAdminExists проверяет, что в базе есть хотя бы один админ.
// Если нет — пишет в лог инструкцию, но не создаёт его автоматически:
// админ должен задать пароль сам, через `gopanel admin create`.
func checkAdminExists(conn *sql.DB, log *slog.Logger) error {
	n, err := auth.CountAdmins(conn)
	if err != nil {
		return err
	}
	if n == 0 {
		log.Warn("в базе нет администратора — создайте его командой: " +
			"gopanel admin create --user admin")
	}
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

// basePathFile возвращает путь к файлу, куда пишется секретный base_path.
// Файл лежит рядом с базой, с правами 0600 — читает только владелец.
func basePathFile(dbPath string) string {
	return filepath.Join(filepath.Dir(dbPath), "base_path")
}

// writeBasePathFile записывает base_path в файл 0600.
// Файл нужен установщику и CI, чтобы не парсить лог сервиса
// (см. пункт 7 аудита — base_path не должен попадать в журнал).
func writeBasePathFile(dbPath, basePath string) error {
	p := basePathFile(dbPath)
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, []byte(basePath+"\n"), 0o600); err != nil {
		return fmt.Errorf("write %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, p); err != nil {
		return fmt.Errorf("rename %s: %w", p, err)
	}
	return nil
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

// buildConfigFromDB собирает конфиг Xray из состояния БД.
func buildConfigFromDB(conn *sql.DB) ([]byte, error) {
	inbounds, err := db.ListInbounds(conn)
	if err != nil {
		return nil, err
	}

	var input xray.BuildInput

	// Инбаунды.
	for _, in := range inbounds {
		links, err := db.ListUsersForInbound(conn, in.ID)
		if err != nil {
			return nil, err
		}

		var users []xray.UserInput
		for _, l := range links {
			u, err := db.GetUser(conn, l.UserID)
			if err != nil {
				return nil, err
			}
			users = append(users, xray.UserInput{
				UserID:         u.ID,
				Name:           u.Name,
				Email:          u.Email,
				Enabled:        u.Enabled,
				CredentialJSON: l.CredentialJSON,
			})
		}

		input.Inbounds = append(input.Inbounds, xray.InboundInput{
			ID:           in.ID,
			Tag:          in.Tag,
			Protocol:     in.Protocol,
			Port:         in.Port,
			Listen:       in.Listen,
			SettingsJSON: in.SettingsJSON,
			StreamJSON:   in.StreamJSON,
			SniffingJSON: in.SniffingJSON,
			Enabled:      in.Enabled,
			Users:        users,
		})
	}

	// Outbound'ы.
	outbounds, err := db.ListOutbounds(conn)
	if err != nil {
		return nil, err
	}
	for _, o := range outbounds {
		input.Outbounds = append(input.Outbounds, xray.OutboundInput{
			Tag:          o.Tag,
			Protocol:     o.Protocol,
			SettingsJSON: o.SettingsJSON,
			StreamJSON:   o.StreamJSON,
			Enabled:      o.Enabled,
		})
	}

	// Routing rules.
	rules, err := db.ListRoutingRules(conn)
	if err != nil {
		return nil, err
	}
	for _, r := range rules {
		input.Rules = append(input.Rules, xray.RoutingRuleInput{
			ID:              r.ID,
			Position:        r.Position,
			RuleTag:         r.RuleTag,
			DomainsJSON:     r.DomainsJSON,
			IPsJSON:         r.IPsJSON,
			PortsJSON:       r.PortsJSON,
			SourceIPsJSON:   r.SourceIPsJSON,
			Network:         r.Network,
			InboundTagsJSON: r.InboundTagsJSON,
			ProtocolJSON:    r.ProtocolJSON,
			OutboundTag:     r.OutboundTag,
			Enabled:         r.Enabled,
		})
	}

	cfg, err := xray.BuildConfig(input)
	if err != nil {
		return nil, err
	}
	return xray.MarshalConfig(cfg)
}

func runBackup(args []string) error {
	fs := flag.NewFlagSet("backup", flag.ContinueOnError)
	var (
		dbPath  = fs.String("db", "", "путь к БД (если пусто — из конфига)")
		outPath = fs.String("out", "", "куда сохранить дамп (по умолчанию рядом с БД)")
		cfgPath = fs.String("config", "", "путь к JSON-конфигу (если пусто — из env GOPANEL_CONFIG)")
	)
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, err := config.Load(*cfgPath, nil)
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

	out := *outPath
	if out == "" {
		dir := filepath.Dir(cfg.DBPath)
		out = filepath.Join(dir, backup.BackupName())
	}

	if err := backup.Dump(context.Background(), conn, out); err != nil {
		return err
	}

	info, _ := os.Stat(out)
	fmt.Printf("backup created: %s (%d bytes)\n", out, info.Size())
	return nil
}

// readPasswordInteractive запрашивает пароль у пользователя без эха.
// Пароль должен быть не короче MinPasswordLen (12 символов).
func readPasswordInteractive(username string) (string, error) {
	fmt.Printf("пароль для %q: ", username)
	p1, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		return "", fmt.Errorf("read password: %w", err)
	}
	fmt.Print("повторите пароль: ")
	p2, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		return "", fmt.Errorf("read password: %w", err)
	}
	if string(p1) != string(p2) {
		return "", fmt.Errorf("пароли не совпадают")
	}
	return string(p1), nil
}
