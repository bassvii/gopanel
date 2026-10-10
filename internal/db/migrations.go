// SPDX-License-Identifier: AGPL-3.0-or-later

package db

import (
	"database/sql"
	"fmt"
)

// migration — один шаг схемы. Версии строго возрастают.
type migration struct {
	version int
	name    string
	sql     string
}

var migrations = []migration{
	{
		version: 1,
		name:    "initial schema",
		sql: `
CREATE TABLE admins (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    username      TEXT    NOT NULL UNIQUE,
    password_hash TEXT    NOT NULL,
    totp_secret   TEXT    NOT NULL DEFAULT '',
    role          TEXT    NOT NULL DEFAULT 'admin',
    created_at    INTEGER NOT NULL
);

CREATE TABLE sessions (
    id         TEXT    PRIMARY KEY,
    admin_id   INTEGER NOT NULL REFERENCES admins(id) ON DELETE CASCADE,
    token_hash TEXT    NOT NULL,
    ip         TEXT    NOT NULL DEFAULT '',
    user_agent TEXT    NOT NULL DEFAULT '',
    last_seen  INTEGER NOT NULL,
    expires_at INTEGER NOT NULL,
    csrf_token TEXT    NOT NULL
);
CREATE INDEX idx_sessions_admin ON sessions(admin_id);

CREATE TABLE settings (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

CREATE TABLE audit_log (
    id       INTEGER PRIMARY KEY AUTOINCREMENT,
    admin_id INTEGER REFERENCES admins(id) ON DELETE SET NULL,
    action   TEXT    NOT NULL,
    target   TEXT    NOT NULL DEFAULT '',
    ip       TEXT    NOT NULL DEFAULT '',
    ts       INTEGER NOT NULL
);
CREATE INDEX idx_audit_ts ON audit_log(ts);
`,
	},
	{
		version: 2,
		name:    "login attempts",
		sql: `
CREATE TABLE login_attempts (
    ip           TEXT    PRIMARY KEY,
    count        INTEGER NOT NULL DEFAULT 0,
    first_failed INTEGER NOT NULL DEFAULT 0,
    locked_until INTEGER NOT NULL DEFAULT 0
);
`,
	},
	{
		version: 3,
		name:    "recovery codes",
		sql: `
CREATE TABLE recovery_codes (
    id        INTEGER PRIMARY KEY AUTOINCREMENT,
    admin_id  INTEGER NOT NULL REFERENCES admins(id) ON DELETE CASCADE,
    code_hash TEXT    NOT NULL,
    used_at   INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX idx_recovery_admin ON recovery_codes(admin_id);
`,
	},
	{
		version: 4,
		name:    "inbounds, users, user_inbound, traffic",
		sql: `
CREATE TABLE inbounds (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    tag           TEXT    NOT NULL UNIQUE,
    protocol      TEXT    NOT NULL,
    port          INTEGER NOT NULL,
    listen        TEXT    NOT NULL DEFAULT '0.0.0.0',
    settings_json TEXT    NOT NULL DEFAULT '{}',
    stream_json   TEXT    NOT NULL DEFAULT '{}',
    sniffing_json TEXT    NOT NULL DEFAULT '{}',
    enabled       INTEGER NOT NULL DEFAULT 1,
    created_at    INTEGER NOT NULL
);
CREATE INDEX idx_inbounds_protocol ON inbounds(protocol);

CREATE TABLE users (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    name           TEXT    NOT NULL UNIQUE,
    email          TEXT    NOT NULL UNIQUE,
    enabled        INTEGER NOT NULL DEFAULT 1,
    quota_bytes    INTEGER NOT NULL DEFAULT 0,
    used_bytes     INTEGER NOT NULL DEFAULT 0,
    expires_at     INTEGER NOT NULL DEFAULT 0,
    device_limit   INTEGER NOT NULL DEFAULT 0,
    sub_token_hash TEXT    NOT NULL DEFAULT '',
    note           TEXT    NOT NULL DEFAULT '',
    created_at     INTEGER NOT NULL
);
CREATE INDEX idx_users_email ON users(email);

CREATE TABLE user_inbound (
    user_id         INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    inbound_id      INTEGER NOT NULL REFERENCES inbounds(id) ON DELETE CASCADE,
    credential_json TEXT    NOT NULL DEFAULT '{}',
    PRIMARY KEY (user_id, inbound_id)
);
CREATE INDEX idx_user_inbound_inbound ON user_inbound(inbound_id);

CREATE TABLE traffic (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    ts         INTEGER NOT NULL,
    up_bytes   INTEGER NOT NULL DEFAULT 0,
    down_bytes INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX idx_traffic_user_ts ON traffic(user_id, ts);
`,
	},
	{
		version: 5,
		name:    "audit action_ru",
		sql: `
ALTER TABLE audit_log ADD COLUMN action_ru TEXT NOT NULL DEFAULT '';
CREATE INDEX idx_audit_action_ru ON audit_log(action_ru);
`,
	},
}

func migrate(conn *sql.DB) error {
	if _, err := conn.Exec(`
CREATE TABLE IF NOT EXISTS schema_migrations (
    version    INTEGER PRIMARY KEY,
    name       TEXT    NOT NULL,
    applied_at INTEGER NOT NULL
);`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	var current int
	row := conn.QueryRow(`SELECT COALESCE(MAX(version), 0) FROM schema_migrations`)
	if err := row.Scan(&current); err != nil {
		return fmt.Errorf("read current version: %w", err)
	}

	for _, m := range migrations {
		if m.version <= current {
			continue
		}
		if err := applyMigration(conn, m); err != nil {
			return fmt.Errorf("migration %d (%s): %w", m.version, m.name, err)
		}
	}
	return nil
}

func applyMigration(conn *sql.DB, m migration) error {
	tx, err := conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(m.sql); err != nil {
		return err
	}
	if _, err := tx.Exec(
		`INSERT INTO schema_migrations (version, name, applied_at) VALUES (?, ?, strftime('%s','now'))`,
		m.version, m.name,
	); err != nil {
		return err
	}
	return tx.Commit()
}
