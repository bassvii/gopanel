// SPDX-License-Identifier: AGPL-3.0-or-later

// Package db отвечает за подключение к SQLite, миграции и общие
// вспомогательные функции для работы с базой.
package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// Open открывает базу, включает WAL, выставляет права 0600 на файл,
// применяет миграции.
func Open(path string) (*sql.DB, error) {
	if err := ensureDir(path); err != nil {
		return nil, err
	}

	dsn := "file:" + path + "?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)"
	conn, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	// Ограничить пул: SQLite плохо переносит много одновременных писателей.
	conn.SetMaxOpenConns(1)

	if err := conn.Ping(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}

	if err := chmod0600(path); err != nil {
		conn.Close()
		return nil, err
	}

	if err := migrate(conn); err != nil {
		conn.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return conn, nil
}

func ensureDir(path string) error {
	dir := filepath.Dir(path)
	if dir == "" || dir == "." {
		return nil
	}
	return os.MkdirAll(dir, 0o700)
}

func chmod0600(path string) error {
	// Файл может ещё не существовать, если SQLite отложил создание.
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil
	}
	return os.Chmod(path, 0o600)
}
