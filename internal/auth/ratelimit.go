// SPDX-License-Identifier: AGPL-3.0-or-later

package auth

import (
	"database/sql"
	"fmt"
	"time"
)

const (
	// MaxFailedAttempts — сколько неудачных попыток входа допускается
	// в пределах окна.
	MaxFailedAttempts = 5
	// FailedAttemptWindow — окно, в котором считаются попытки.
	FailedAttemptWindow = 15 * time.Minute
	// LockoutDuration — на сколько блокируется IP после превышения.
	LockoutDuration = 15 * time.Minute
)

// IsLockedOut проверяет, заблокирован ли IP для входа.
func IsLockedOut(conn *sql.DB, ip string) (bool, time.Duration, error) {
	var lockedUntil int64
	err := conn.QueryRow(
		`SELECT locked_until FROM login_attempts WHERE ip = ?`,
		ip,
	).Scan(&lockedUntil)
	if err == sql.ErrNoRows {
		return false, 0, nil
	}
	if err != nil {
		return false, 0, fmt.Errorf("get lockout: %w", err)
	}

	now := time.Now().Unix()
	if lockedUntil > now {
		return true, time.Duration(lockedUntil-now) * time.Second, nil
	}
	return false, 0, nil
}

// RecordFailedAttempt увеличивает счётчик неудач для IP.
// Если превышен порог — выставляет блокировку.
func RecordFailedAttempt(conn *sql.DB, ip string) error {
	now := time.Now().Unix()
	windowStart := now - int64(FailedAttemptWindow.Seconds())

	tx, err := conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var (
		count       int
		firstFailed int64
	)
	err = tx.QueryRow(
		`SELECT count, first_failed FROM login_attempts WHERE ip = ?`,
		ip,
	).Scan(&count, &firstFailed)
	if err == sql.ErrNoRows {
		if _, err := tx.Exec(
			`INSERT INTO login_attempts (ip, count, first_failed, locked_until)
			 VALUES (?, 1, ?, 0)`,
			ip, now,
		); err != nil {
			return err
		}
		return tx.Commit()
	}
	if err != nil {
		return err
	}

	// Окно истекло — начинаем заново.
	if firstFailed < windowStart {
		if _, err := tx.Exec(
			`UPDATE login_attempts SET count = 1, first_failed = ?, locked_until = 0 WHERE ip = ?`,
			now, ip,
		); err != nil {
			return err
		}
		return tx.Commit()
	}

	count++
	var lockedUntil int64
	if count >= MaxFailedAttempts {
		lockedUntil = now + int64(LockoutDuration.Seconds())
	}

	if _, err := tx.Exec(
		`UPDATE login_attempts SET count = ?, locked_until = ? WHERE ip = ?`,
		count, lockedUntil, ip,
	); err != nil {
		return err
	}
	return tx.Commit()
}

// ClearFailedAttempts сбрасывает счётчик после успешного входа.
func ClearFailedAttempts(conn *sql.DB, ip string) error {
	_, err := conn.Exec(`DELETE FROM login_attempts WHERE ip = ?`, ip)
	return err
}
