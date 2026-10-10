// SPDX-License-Identifier: AGPL-3.0-or-later

// Package api содержит админский HTTP-listener и middleware.
// Listener привязан к loopback: снаружи панель недоступна.
package api

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"
)

// Server — админский HTTP-сервер.
type Server struct {
	cfg    Config
	db     *sql.DB
	log    *slog.Logger
	http   *http.Server
	mux    *http.ServeMux

	secret   string
	addr     string
	port     int
	reloadCh chan struct{}
	webFS    fs.FS
}

// Config — параметры сервера.
type Config struct {
	Listen   string
	Port     int
	BasePath string
}

// New создаёт сервер.
func New(cfg Config, conn *sql.DB, log *slog.Logger) (*Server, error) {
	if cfg.Listen == "" {
		return nil, errors.New("listen address is required")
	}
	ip := net.ParseIP(cfg.Listen)
	if ip == nil || !ip.IsLoopback() {
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

	s := &Server{
		cfg:      cfg,
		db:       conn,
		log:      log,
		mux:      http.NewServeMux(),
		secret:   secret,
		reloadCh: make(chan struct{}, 1),
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

// ReloadCh возвращает канал для чтения сигналов перезагрузки.
func (s *Server) ReloadCh() <-chan struct{} {
	return s.reloadCh
}

// ReloadSignal возвращает канал для отправки сигналов перезагрузки.
func (s *Server) ReloadSignal() chan<- struct{} {
	return s.reloadCh
}

// requestReload отправляет сигнал перегенерировать конфиг Xray.
func (s *Server) requestReload() {
	select {
	case s.reloadCh <- struct{}{}:
	default:
	}
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

	s.log.Info("admin listener started",
		"addr", s.addr,
		"base_path", s.secret,
	)

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

// routes регистрирует все маршруты. Вызывается при создании
// и при SetWebFS (чтобы перерегистрировать корневой маршрут).
func (s *Server) routes() {
	s.mux = http.NewServeMux()

	s.mux.HandleFunc(s.secret+"/login", s.handleLogin)
	s.mux.HandleFunc(s.secret+"/logout", s.handleLogout)
	s.mux.HandleFunc(s.secret+"/me", s.handleMe)
	s.mux.HandleFunc(s.secret+"/2fa/setup", s.handleTOTPSetup)
	s.mux.HandleFunc(s.secret+"/2fa/qr", s.handleTOTPQR)
	s.mux.HandleFunc(s.secret+"/2fa/enable", s.handleTOTPEnable)
	s.mux.HandleFunc(s.secret+"/2fa/disable", s.handleTOTPDisable)
	s.mux.HandleFunc(s.secret+"/2fa/status", s.handleTOTPStatus)
	s.mux.HandleFunc(s.secret+"/audit", s.handleAudit)
	s.mux.HandleFunc(s.secret+"/inbounds", s.handleInbounds)
	s.mux.HandleFunc(s.secret+"/inbounds/", s.handleInbound)
	s.mux.HandleFunc(s.secret+"/users", s.handleUsers)
	s.mux.HandleFunc(s.secret+"/users/", s.handleUserRoutes)
	s.mux.HandleFunc(s.secret+"/stats/summary", s.handleStatsSummary)
	s.mux.HandleFunc(s.secret+"/me/password", s.handleChangePassword)
	s.mux.HandleFunc(s.secret+"/me/sessions", s.handleSessions)
	s.mux.HandleFunc(s.secret+"/me/sessions/", s.handleSession)

	if s.webFS != nil {
		s.mux.HandleFunc("/", s.handleStatic)
	} else {
		s.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			http.NotFound(w, r)
		})
	}
}

// handleStatic отдаёт файлы фронтенда. Если файл не найден —
// отдаёт index.html (SPA-роутинг).
func (s *Server) handleStatic(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/")
	if path == "" {
		path = "index.html"
	}
	f, err := s.webFS.Open(path)
	if err != nil {
		s.serveIndex(w, r)
		return
	}
	f.Close()
	http.FileServer(http.FS(s.webFS)).ServeHTTP(w, r)
}

// serveIndex отдаёт index.html.
func (s *Server) serveIndex(w http.ResponseWriter, r *http.Request) {
	data, err := fs.ReadFile(s.webFS, "index.html")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(data)
}

// handleUserRoutes распределяет /users/{id}, /users/{id}/inbounds, /users/{id}/links.
func (s *Server) handleUserRoutes(w http.ResponseWriter, r *http.Request) {
	switch {
	case strings.HasSuffix(r.URL.Path, "/inbounds"):
		s.handleUserInbounds(w, r)
	case strings.HasSuffix(r.URL.Path, "/links"):
		s.handleUserLinks(w, r)
	default:
		s.handleUser(w, r)
	}
}
