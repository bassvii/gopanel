// SPDX-License-Identifier: AGPL-3.0-or-later

package db

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// User — пользователь панели.
type User struct {
	ID           int64
	Name         string
	Email        string
	Enabled      bool
	QuotaBytes   int64
	UsedBytes    int64
	ExpiresAt    time.Time
	DeviceLimit  int
	SubTokenHash string
	Note         string
	CreatedAt    time.Time
}

// CreateUser создаёт пользователя.
// Если Email пуст — генерируется из Name.
func CreateUser(conn *sql.DB, u User) (int64, error) {
	u.Name = strings.TrimSpace(u.Name)
	if u.Name == "" {
		return 0, errors.New("name is required")
	}
	if u.Email == "" {
		u.Email = u.Name + "@gopanel"
	}

	var expires int64
	if !u.ExpiresAt.IsZero() {
		expires = u.ExpiresAt.Unix()
	}

	res, err := conn.Exec(
		`INSERT INTO users
		 (name, email, enabled, quota_bytes, used_bytes, expires_at,
		  device_limit, sub_token_hash, note, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		u.Name, u.Email, boolToInt(u.Enabled),
		u.QuotaBytes, u.UsedBytes, expires,
		u.DeviceLimit, u.SubTokenHash, u.Note, time.Now().Unix(),
	)
	if err != nil {
		return 0, fmt.Errorf("insert user: %w", err)
	}
	return res.LastInsertId()
}

// GetUser возвращает пользователя по id.
func GetUser(conn *sql.DB, id int64) (User, error) {
	var (
		u         User
		enabled   int
		expires   int64
		createdAt int64
	)
	err := conn.QueryRow(
		`SELECT id, name, email, enabled, quota_bytes, used_bytes, expires_at,
		        device_limit, sub_token_hash, note, created_at
		 FROM users WHERE id = ?`,
		id,
	).Scan(&u.ID, &u.Name, &u.Email, &enabled, &u.QuotaBytes, &u.UsedBytes,
		&expires, &u.DeviceLimit, &u.SubTokenHash, &u.Note, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return u, ErrNotFound
	}
	if err != nil {
		return u, fmt.Errorf("get user: %w", err)
	}
	u.Enabled = enabled != 0
	if expires > 0 {
		u.ExpiresAt = time.Unix(expires, 0)
	}
	u.CreatedAt = time.Unix(createdAt, 0)
	return u, nil
}

// ListUsers возвращает всех пользователей (в порядке создания).
func ListUsers(conn *sql.DB) ([]User, error) {
	rows, err := conn.Query(
		`SELECT id, name, email, enabled, quota_bytes, used_bytes, expires_at,
		        device_limit, sub_token_hash, note, created_at
		 FROM users ORDER BY id`,
	)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()

	var out []User
	for rows.Next() {
		var (
			u         User
			enabled   int
			expires   int64
			createdAt int64
		)
		if err := rows.Scan(&u.ID, &u.Name, &u.Email, &enabled, &u.QuotaBytes,
			&u.UsedBytes, &expires, &u.DeviceLimit, &u.SubTokenHash, &u.Note,
			&createdAt); err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}
		u.Enabled = enabled != 0
		if expires > 0 {
			u.ExpiresAt = time.Unix(expires, 0)
		}
		u.CreatedAt = time.Unix(createdAt, 0)
		out = append(out, u)
	}
	return out, rows.Err()
}

// UpdateUser обновляет пользователя.
func UpdateUser(conn *sql.DB, u User) error {
	if u.ID == 0 {
		return errors.New("id is required")
	}
	var expires int64
	if !u.ExpiresAt.IsZero() {
		expires = u.ExpiresAt.Unix()
	}
	_, err := conn.Exec(
		`UPDATE users
		 SET name = ?, email = ?, enabled = ?, quota_bytes = ?, used_bytes = ?,
		     expires_at = ?, device_limit = ?, sub_token_hash = ?, note = ?
		 WHERE id = ?`,
		u.Name, u.Email, boolToInt(u.Enabled), u.QuotaBytes, u.UsedBytes,
		expires, u.DeviceLimit, u.SubTokenHash, u.Note, u.ID,
	)
	if err != nil {
		return fmt.Errorf("update user: %w", err)
	}
	return nil
}

// DeleteUser удаляет пользователя (связи и трафик — каскадом).
func DeleteUser(conn *sql.DB, id int64) error {
	_, err := conn.Exec(`DELETE FROM users WHERE id = ?`, id)
	return err
}
