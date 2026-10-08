// SPDX-License-Identifier: AGPL-3.0-or-later

package auth

import (
	"database/sql"
	"errors"
	"fmt"
)

// ErrAdminNotFound возвращается, когда админа с таким именем нет.
var ErrAdminNotFound = errors.New("admin not found")

// ResetPassword меняет пароль, отключает 2FA и удаляет все сессии
// администратора. Используется в аварийном CLI.
func ResetPassword(conn *sql.DB, username, newPassword string) error {
	hash, err := HashPassword(newPassword)
	if err != nil {
		return err
	}

	tx, err := conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var adminID int64
	err = tx.QueryRow(`SELECT id FROM admins WHERE username = ?`, username).Scan(&adminID)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrAdminNotFound
	}
	if err != nil {
		return fmt.Errorf("find admin: %w", err)
	}

	if _, err := tx.Exec(
		`UPDATE admins SET password_hash = ?, totp_secret = '' WHERE id = ?`,
		hash, adminID,
	); err != nil {
		return fmt.Errorf("update admin: %w", err)
	}

	if _, err := tx.Exec(`DELETE FROM sessions WHERE admin_id = ?`, adminID); err != nil {
		return fmt.Errorf("delete sessions: %w", err)
	}

	if _, err := tx.Exec(`DELETE FROM recovery_codes WHERE admin_id = ?`, adminID); err != nil {
		return fmt.Errorf("delete recovery codes: %w", err)
	}

	return tx.Commit()
}
