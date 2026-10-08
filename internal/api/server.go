// SPDX-License-Identifier: AGPL-3.0-or-later

// Package api содержит админский HTTP-listener и middleware.
// Listener привязан к loopback: снаружи панель недоступна.
package api

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"
)

// Server — админский HTTP-сервер.
type Server struct {
	cfg  Config
	db   *sql.DB
	log  *slog.Logger
	http *http.Server
	mux  *http.ServeMux

	secret string
	addr   string
	port   int
}

// Config — параметры сервера, отдельно от config.Config,
// чтобы пакет api не зависел от всего конфига панели.
type Config struct {
	Listen   string
	Port     int
	BasePath string
}

// New создаёт сервер. BasePath должен быть уже готов (main решает,
// сгенерировать новый или взять из БД). Port == 0 означает
// «выбрать свободный при Start».
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
		cfg:    cfg,
		db:     conn,
		log:    log,
		mux:    http.NewServeMux(),
		secret: secret,
	}
	s.routes()
	return s, nil
}

// Addr возвращает фактический адрес после Start.
func (s *Server) Addr() string { return s.addr }

// Port возвращает фактический порт после Start.
func (s *Server) Port() int { return s.port }

// SecretPath возвращает базовый путь.
func (s *Server) SecretPath() string { return s.secret }

// Start запускает сервер и блокируется до Stop или ошибки.
// Если cfg.Port == 0, ОС выберет свободный порт; фактический
// доступен через Port().
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

func (s *Server) routes() {
	// Всё под секретным базовым путём.
	s.mux.HandleFunc(s.secret+"/login", s.handleLogin)
	s.mux.HandleFunc(s.secret+"/logout", s.handleLogout)
	s.mux.HandleFunc(s.secret+"/me", s.handleMe)
	s.mux.HandleFunc(s.secret+"/2fa/setup", s.handleTOTPSetup)
	s.mux.HandleFunc(s.secret+"/2fa/qr", s.handleTOTPQR)
	s.mux.HandleFunc(s.secret+"/2fa/enable", s.handleTOTPEnable)
	s.mux.HandleFunc(s.secret+"/2fa/disable", s.handleTOTPDisable)
	s.mux.HandleFunc(s.secret+"/2fa/status", s.handleTOTPStatus)

	// Всё остальное — 404.
	s.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
}
