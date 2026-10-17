// SPDX-License-Identifier: AGPL-3.0-or-later

// Package api содержит админский HTTP-listener и middleware.
// Listener привязан к loopback: снаружи панель недоступна.
package api

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"mime"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/bassvii/gopanel/internal/core/xray"
	"github.com/bassvii/gopanel/internal/notify"
	"github.com/bassvii/gopanel/internal/system"
)

// Server — админский HTTP-сервер.
type Server struct {
	cfg       Config
	db        *sql.DB
	log       *slog.Logger
	http      *http.Server
	mux       *http.ServeMux
	core      *xray.Core
	system    *system.Collector
	webFS     fs.FS
	startTime time.Time

	secret            string
	addr              string
	port              int
	cachedXrayVersion string
	dbPath            string

	// trustedNets — распарсенные CIDR'ы из cfg.TrustedProxies.
	// Пустой, если доверенных прокси нет.
	trustedNets []*net.IPNet

	// applier — замыкание, которое собирает конфиг Xray из БД
	// и применяет его. Устанавливается из main.go через SetApplier.
	// Возвращает ошибку, если xray -test упал или процесс не стартовал.
	applier func() error
	// applyMu сериализует вызовы applier: одновременно два apply
	// (например, из двух хендлеров) не должны запускать Xray.
	applyMu sync.Mutex
}

// Config — параметры сервера.
type Config struct {
	Listen           string
	Port             int
	BasePath         string
	AllowNonLoopback bool
	// SecureCookies выставляет Secure на cookie сессии.
	SecureCookies bool
	// TrustedProxies — CIDR'ы, от которых доверяем X-Forwarded-For.
	// Пустой список = доверяем только RemoteAddr.
	TrustedProxies []string
}

// New создаёт сервер.
func New(cfg Config, conn *sql.DB, log *slog.Logger) (*Server, error) {
	if cfg.Listen == "" {
		return nil, errors.New("listen address is required")
	}
	ip := net.ParseIP(cfg.Listen)
	if ip == nil {
		return nil, fmt.Errorf("invalid listen address: %q", cfg.Listen)
	}
	if !ip.IsLoopback() && !cfg.AllowNonLoopback {
		return nil, fmt.Errorf("admin listener must be on loopback, got %q", cfg.Listen)
	}

	if cfg.Port < 0 || cfg.Port > 65535 {
		return nil, fmt.Errorf("port must be 0..65535, got %d", cfg.Port)
	}
	if cfg.BasePath == "" {
		return nil, errors.New("base path is required")
	}

	secret := cfg.BasePath
	if secret[0] != '/' {
		secret = "/" + secret
	}

	// Парсим доверенные прокси.
	var trustedNets []*net.IPNet
	for _, cidr := range cfg.TrustedProxies {
		cidr = strings.TrimSpace(cidr)
		if cidr == "" {
			continue
		}
		// Поддерживаем как CIDR (10.0.0.0/8), так и одиночный IP
		// (127.0.0.1 → 127.0.0.1/32).
		if !strings.Contains(cidr, "/") {
			if ip := net.ParseIP(cidr); ip != nil {
				if ip.To4() != nil {
					cidr += "/32"
				} else {
					cidr += "/128"
				}
			}
		}
		_, ipnet, err := net.ParseCIDR(cidr)
		if err != nil {
			return nil, fmt.Errorf("invalid trusted proxy %q: %w", cidr, err)
		}
		trustedNets = append(trustedNets, ipnet)
	}

	s := &Server{
		cfg:         cfg,
		db:          conn,
		log:         log,
		mux:         http.NewServeMux(),
		secret:      secret,
		trustedNets: trustedNets,
	}
	s.routes()
	return s, nil
}

// SetWebFS устанавливает файловую систему с фронтендом
// и перерегистрирует маршруты.
func (s *Server) SetWebFS(webFS fs.FS) {
	s.webFS = webFS
	s.routes()
}

// Addr возвращает фактический адрес после Start.
func (s *Server) Addr() string { return s.addr }

// Port возвращает фактический порт после Start.
func (s *Server) Port() int { return s.port }

// SecretPath возвращает базовый путь.
func (s *Server) SecretPath() string { return s.secret }

// SetApplier устанавливает замыкание, которое собирает конфиг Xray
// из БД и применяет его (с проверкой xray -test). Вызывается из
// main.go один раз при старте.
func (s *Server) SetApplier(f func() error) {
	s.applier = f
}

// ApplyNow — экспортированная версия applyNow для внешних вызывающих
// (лимитер, reloadLoop). Возвращает ошибку применения конфига.
func (s *Server) ApplyNow() error {
	return s.applyNow()
}

func (s *Server) applyNow() error {
	if s.applier == nil {
		// Applier не установлен — нечего применять.
		return nil
	}
	s.applyMu.Lock()
	defer s.applyMu.Unlock()

	if err := s.applier(); err != nil {
		s.log.Error("apply config failed", "err", err)
		notify.XrayFailed(s.db, s.log, err.Error())
		return err
	}
	return nil
}

// Start запускает сервер и блокируется до Stop или ошибки.
func (s *Server) Start() error {
	ln, err := net.Listen("tcp", fmt.Sprintf("%s:%d", s.cfg.Listen, s.cfg.Port))
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}

	s.addr = ln.Addr().String()
	if tcp, ok := ln.Addr().(*net.TCPAddr); ok {
		s.port = tcp.Port
	}

	s.http = &http.Server{
		Handler:           s.withMiddleware(s.mux),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// Предупреждаем, если панель не на loopback и cookie без Secure.
	// Это типичный сценарий «выставил панель наружу без HTTPS» —
	// cookie уйдёт по HTTP. См. P2-4 третьего аудита.
	if !isLoopbackAddr(s.cfg.Listen) {
		if !s.cfg.SecureCookies {
			s.log.Warn("admin listener is not on loopback, but SecureCookies is false",
				"listen", s.cfg.Listen,
				"hint", "включите --secure-cookies и поставьте панель за HTTPS-прокси, "+
					"или верните listener на 127.0.0.1")
		}
		if len(s.trustedNets) == 0 {
			s.log.Warn("admin listener is not on loopback, but TrustedProxies is empty",
				"listen", s.cfg.Listen,
				"hint", "без TrustedProxies все запросы будут выглядеть с одного IP (адрес прокси)")
		}
	}

	s.log.Info("admin listener started", "addr", s.addr)

	err = s.http.Serve(ln)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

// Stop корректно останавливает сервер.
func (s *Server) Stop(ctx context.Context) error {
	if s.http == nil {
		return nil
	}
	return s.http.Shutdown(ctx)
}

// SetCore устанавливает ядро Xray (для API генерации ключей).
func (s *Server) SetCore(c *xray.Core) {
	s.core = c
}

// routes регистрирует все маршруты. Вызывается при создании
// и при SetWebFS (чтобы перерегистрировать корневой маршрут).
func (s *Server) routes() {
	s.mux = http.NewServeMux()

	// --- Аутентификация ---
	s.mux.HandleFunc("POST "+s.secret+"/login", s.handleLogin)
	s.mux.HandleFunc("POST "+s.secret+"/logout", s.handleLogout)
	s.mux.HandleFunc("GET "+s.secret+"/me", s.handleMe)

	// --- 2FA ---
	s.mux.HandleFunc("POST "+s.secret+"/2fa/setup", s.handleTOTPSetup)
	s.mux.HandleFunc("GET "+s.secret+"/2fa/qr", s.handleTOTPQR)
	s.mux.HandleFunc("POST "+s.secret+"/2fa/enable", s.handleTOTPEnable)
	s.mux.HandleFunc("POST "+s.secret+"/2fa/disable", s.handleTOTPDisable)
	s.mux.HandleFunc("GET "+s.secret+"/2fa/status", s.handleTOTPStatus)

	// --- Аудит ---
	s.mux.HandleFunc("GET "+s.secret+"/audit", s.handleAudit)

	// --- Inbounds ---
	s.mux.HandleFunc("GET "+s.secret+"/inbounds", s.handleInbounds)
	s.mux.HandleFunc("POST "+s.secret+"/inbounds", s.handleInbounds)
	s.mux.HandleFunc("GET "+s.secret+"/inbounds/{id}", s.handleInbound)
	s.mux.HandleFunc("PUT "+s.secret+"/inbounds/{id}", s.handleInbound)
	s.mux.HandleFunc("DELETE "+s.secret+"/inbounds/{id}", s.handleInbound)

	// --- Users ---
	s.mux.HandleFunc("GET "+s.secret+"/users", s.handleUsers)
	s.mux.HandleFunc("POST "+s.secret+"/users", s.handleUsers)
	s.mux.HandleFunc("GET "+s.secret+"/users/{id}", s.handleUser)
	s.mux.HandleFunc("PUT "+s.secret+"/users/{id}", s.handleUser)
	s.mux.HandleFunc("DELETE "+s.secret+"/users/{id}", s.handleUser)
	s.mux.HandleFunc("POST "+s.secret+"/users/{id}/inbounds", s.handleUserInbounds)
	s.mux.HandleFunc("GET "+s.secret+"/users/{id}/links", s.handleUserLinks)
	s.mux.HandleFunc("POST "+s.secret+"/users/{id}/reset-traffic", s.handleUserResetTraffic)

	// --- Stats ---
	s.mux.HandleFunc("GET "+s.secret+"/stats/summary", s.handleStatsSummary)

	// --- Me ---
	s.mux.HandleFunc("POST "+s.secret+"/me/password", s.handleChangePassword)
	s.mux.HandleFunc("GET "+s.secret+"/me/sessions", s.handleSessions)
	s.mux.HandleFunc("DELETE "+s.secret+"/me/sessions", s.handleSessions)
	s.mux.HandleFunc("DELETE "+s.secret+"/me/sessions/{id}", s.handleSession)

	// --- Backup ---
	s.mux.HandleFunc("POST "+s.secret+"/backup/download", s.handleBackupDownload)

	// --- Xray helpers ---
	s.mux.HandleFunc("POST "+s.secret+"/xray/x25519", s.handleX25519)
	s.mux.HandleFunc("POST "+s.secret+"/xray/shortid", s.handleShortID)
	s.mux.HandleFunc("POST "+s.secret+"/xray/vlessenc", s.handleVlessEnc)
	s.mux.HandleFunc("POST "+s.secret+"/xray/tls-cert", s.handleTLSCert)

	// --- System ---
	s.mux.HandleFunc("GET "+s.secret+"/system/stats", s.handleSystemStats)
	s.mux.HandleFunc("GET "+s.secret+"/system/geo/status", s.handleGeoStatus)
	s.mux.HandleFunc("POST "+s.secret+"/system/geo/update", s.handleGeoUpdate)

	// --- Outbounds ---
	s.mux.HandleFunc("GET "+s.secret+"/outbounds", s.handleOutbounds)
	s.mux.HandleFunc("POST "+s.secret+"/outbounds", s.handleOutbounds)
	s.mux.HandleFunc("GET "+s.secret+"/outbounds/{id}", s.handleOutbound)
	s.mux.HandleFunc("PUT "+s.secret+"/outbounds/{id}", s.handleOutbound)
	s.mux.HandleFunc("DELETE "+s.secret+"/outbounds/{id}", s.handleOutbound)

	// --- Routing ---
	s.mux.HandleFunc("GET "+s.secret+"/routing/rules", s.handleRoutingRules)
	s.mux.HandleFunc("POST "+s.secret+"/routing/rules", s.handleRoutingRules)
	s.mux.HandleFunc("GET "+s.secret+"/routing/rules/{id}", s.handleRoutingRule)
	s.mux.HandleFunc("PUT "+s.secret+"/routing/rules/{id}", s.handleRoutingRule)
	s.mux.HandleFunc("DELETE "+s.secret+"/routing/rules/{id}", s.handleRoutingRule)
	s.mux.HandleFunc("POST "+s.secret+"/routing/rules/{id}/move", s.handleRoutingRuleMoveRoute)
	s.mux.HandleFunc("GET "+s.secret+"/routing/presets", s.handleRoutingPresets)
	s.mux.HandleFunc("POST "+s.secret+"/routing/presets/{name}", s.handleRoutingPresetApply)

	// --- Notifications ---
	s.mux.HandleFunc("GET "+s.secret+"/notifications", s.handleNotifications)
	s.mux.HandleFunc("POST "+s.secret+"/notifications/read", s.handleNotificationsRead)

	// --- Healthcheck ---
	// Публичный эндпоинт без авторизации для Docker HEALTHCHECK.
	// Отвечает 200 OK и "ok", не раскрывает версий и путей.
	// Живёт в корне, вне base_path — чтобы Docker мог проверить
	// контейнер, не зная секретного префикса.
	s.mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	// --- Статика ---
	if s.webFS != nil {
		s.mux.HandleFunc("GET "+s.secret+"/", s.handleStatic)
	}
	s.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
}

// handleStatic отдаёт файлы фронтенда. Если файл не найден —
// отдаёт index.html (SPA-роутинг).
//
// index.html получает подстановку base_path: placeholder
// __GOPANEL_BASE__ заменяется на s.secret. Так фронт знает
// секретный префикс для любого base_path, а не только hex-32.
// См. #6 нового аудита.
func (s *Server) handleStatic(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, s.secret)
	path = strings.TrimPrefix(path, "/")
	if path == "" {
		path = "index.html"
	}

	data, err := fs.ReadFile(s.webFS, path)
	if err != nil {
		s.serveIndex(w, r)
		return
	}

	// index.html — подставляем base_path.
	if path == "index.html" {
		data = s.injectBasePath(data)
	}

	ctype := mime.TypeByExtension(filepath.Ext(path))
	if ctype != "" {
		w.Header().Set("Content-Type", ctype)
	}
	_, _ = w.Write(data)
}

// serveIndex отдаёт index.html с подставленным base_path.
func (s *Server) serveIndex(w http.ResponseWriter, r *http.Request) {
	data, err := fs.ReadFile(s.webFS, "index.html")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	data = s.injectBasePath(data)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(data)
}

// injectBasePath заменяет placeholder __GOPANEL_BASE__ в index.html
// на s.secret. Фронт читает его из <meta name="gopanel-base">.
func (s *Server) injectBasePath(data []byte) []byte {
	return bytes.ReplaceAll(data, []byte("__GOPANEL_BASE__"), []byte(s.secret))
}

// SetSystemCollector устанавливает сборщик метрик системы.
func (s *Server) SetSystemCollector(c *system.Collector, dbPath string, startTime time.Time) {
	s.system = c
	s.dbPath = dbPath
	s.startTime = startTime
}

func (s *Server) SetXrayVersion(v string) {
	s.cachedXrayVersion = v
}

// isLoopbackAddr возвращает true, если addr — loopback (127.0.0.1
// или ::1) или пустой (тогда listener упадёт позже, не наша забота).
func isLoopbackAddr(addr string) bool {
	if addr == "" {
		return false
	}
	ip := net.ParseIP(addr)
	if ip == nil {
		return false
	}
	return ip.IsLoopback()
}
