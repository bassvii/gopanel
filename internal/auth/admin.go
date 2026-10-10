// SPDX-License-Identifier: AGPL-3.0-or-later

package auth

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ErrNoAdmins возвращается, когда в базе ещё нет ни одного админа.
var ErrNoAdmins = errors.New("no admins in database")

// Admin — запись администратора.
type Admin struct {
	ID           int64
	Username     string
	PasswordHash string
	TOTPSecret   string
	Role         string
	CreatedAt    time.Time
}

// CountAdmins возвращает число админов в базе.
func CountAdmins(conn *sql.DB) (int, error) {
	var n int
	if err := conn.QueryRow(`SELECT COUNT(*) FROM admins`).Scan(&n); err != nil {
		return 0, fmt.Errorf("count admins: %w", err)
	}
	return n, nil
}

// CreateAdmin создаёт админа с хешированным паролем.
func CreateAdmin(conn *sql.DB, username, password string) (int64, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return 0, errors.New("username is required")
	}

	hash, err := HashPassword(password)
	if err != nil {
		return 0, err
	}

	res, err := conn.Exec(
		`INSERT INTO admins (username, password_hash, role, created_at)
		 VALUES (?, ?, 'admin', ?)`,
		username, hash, time.Now().Unix(),
	)
	if err != nil {
		return 0, fmt.Errorf("insert admin: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("last insert id: %w", err)
	}
	return id, nil
}

// GetAdminByUsername возвращает админа по имени.
func GetAdminByUsername(conn *sql.DB, username string) (Admin, error) {
	var a Admin
	var createdAt int64
	err := conn.QueryRow(
		`SELECT id, username, password_hash, totp_secret, role, created_at
		 FROM admins WHERE username = ?`,
		username,
	).Scan(&a.ID, &a.Username, &a.PasswordHash, &a.TOTPSecret, &a.Role, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return a, sql.ErrNoRows
	}
	if err != nil {
		return a, fmt.Errorf("get admin: %w", err)
	}
	a.CreatedAt = time.Unix(createdAt, 0)
	return a, nil
}

// ChangePassword меняет пароль администратора, проверяя старый.
func ChangePassword(conn *sql.DB, adminID int64, oldPassword, newPassword string) error {
	admin, err := GetAdminByID(conn, adminID)
	if err != nil {
		return err
	}
	if err := VerifyPassword(oldPassword, admin.PasswordHash); err != nil {
		return errors.New("old password is incorrect")
	}
	hash, err := HashPassword(newPassword)
	if err != nil {
		return err
	}
	_, err = conn.Exec(`UPDATE admins SET password_hash = ? WHERE id = ?`, hash, adminID)
	if err != nil {
		return fmt.Errorf("update password: %w", err)
	}
	return nil
}

// GetAdminByID возвращает админа по ID.
func GetAdminByID(conn *sql.DB, id int64) (Admin, error) {
	var a Admin
	var createdAt int64
	err := conn.QueryRow(
		`SELECT id, username, password_hash, totp_secret, role, created_at
		 FROM admins WHERE id = ?`,
		id,
	).Scan(&a.ID, &a.Username, &a.PasswordHash, &a.TOTPSecret, &a.Role, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return a, sql.ErrNoRows
	}
	if err != nil {
		return a, fmt.Errorf("get admin: %w", err)
	}
	a.CreatedAt = time.Unix(createdAt, 0)
	return a, nil
}
