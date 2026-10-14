// SPDX-License-Identifier: AGPL-3.0-or-later

package auth

import (
	"database/sql"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	conn, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	_, err = conn.Exec(`
CREATE TABLE sessions (
    id         TEXT    PRIMARY KEY,
    admin_id   INTEGER NOT NULL,
    token_hash TEXT    NOT NULL,
    ip         TEXT    NOT NULL DEFAULT '',
    user_agent TEXT    NOT NULL DEFAULT '',
    last_seen  INTEGER NOT NULL,
    expires_at INTEGER NOT NULL,
    csrf_token TEXT    NOT NULL
);`)
	if err != nil {
		t.Fatal(err)
	}
	return conn
}

func TestGetSession_IdleTimeout(t *testing.T) {
	conn := setupTestDB(t)
	defer conn.Close()

	token, sess, err := NewSession(conn, 1, "127.0.0.1", "test")
	if err != nil {
		t.Fatal(err)
	}

	// Свежая сессия — работает.
	if _, err := GetSession(conn, token); err != nil {
		t.Fatalf("fresh session rejected: %v", err)
	}

	// Сдвигаем last_seen в прошлое больше, чем SessionIdleTimeout.
	past := time.Now().Add(-SessionIdleTimeout - time.Minute).Unix()
	if _, err := conn.Exec(`UPDATE sessions SET last_seen = ? WHERE id = ?`, past, sess.ID); err != nil {
		t.Fatal(err)
	}

	if _, err := GetSession(conn, token); err == nil {
		t.Fatal("idle session should be rejected")
	}

	// И должна быть удалена из БД.
	var count int
	if err := conn.QueryRow(`SELECT COUNT(*) FROM sessions WHERE id = ?`, sess.ID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("idle session not deleted, count=%d", count)
	}
}

func TestGetSession_AbsoluteTTL(t *testing.T) {
	conn := setupTestDB(t)
	defer conn.Close()

	token, sess, err := NewSession(conn, 1, "127.0.0.1", "test")
	if err != nil {
		t.Fatal(err)
	}

	// expires_at в прошлом.
	past := time.Now().Add(-time.Hour).Unix()
	if _, err := conn.Exec(`UPDATE sessions SET expires_at = ? WHERE id = ?`, past, sess.ID); err != nil {
		t.Fatal(err)
	}

	if _, err := GetSession(conn, token); err == nil {
		t.Fatal("expired session should be rejected")
	}
}
