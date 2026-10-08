// SPDX-License-Identifier: AGPL-3.0-or-later

// Package audit записывает действия администраторов.
package audit

import (
	"database/sql"
	"fmt"
	"time"
)

// Event — одно действие в журнале.
type Event struct {
	ID        int64
	AdminID   int64
	Action    string
	Target    string
	IP        string
	Timestamp time.Time
}

// Константы действий. Используются в коде панели.
const (
	ActionLogin          = "login"
	ActionLogout         = "logout"
	ActionLoginFailed    = "login_failed"
	ActionTOTPEnabled    = "totp_enabled"
	ActionTOTPDisabled   = "totp_disabled"
	ActionRecoveryUsed   = "recovery_code_used"
	ActionPasswordReset  = "password_reset"
)

// Log записывает событие. adminID == 0 означает «без администратора»
// (например, неудачный вход по несуществующему логину).
func Log(conn *sql.DB, adminID int64, action, target, ip string) error {
	_, err := conn.Exec(
		`INSERT INTO audit_log (admin_id, action, target, ip, ts)
		 VALUES (?, ?, ?, ?, ?)`,
		nullableAdminID(adminID), action, target, ip, time.Now().Unix(),
	)
	if err != nil {
		return fmt.Errorf("write audit log: %w", err)
	}
	return nil
}

// List возвращает последние N записей журнала (по убыванию времени).
func List(conn *sql.DB, limit int) ([]Event, error) {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	rows, err := conn.Query(
		`SELECT id, COALESCE(admin_id, 0), action, target, ip, ts
		 FROM audit_log ORDER BY ts DESC LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list audit log: %w", err)
	}
	defer rows.Close()

	var events []Event
	for rows.Next() {
		var (
			e  Event
			ts int64
		)
		if err := rows.Scan(&e.ID, &e.AdminID, &e.Action, &e.Target, &e.IP, &ts); err != nil {
			return nil, fmt.Errorf("scan audit log: %w", err)
		}
		e.Timestamp = time.Unix(ts, 0)
		events = append(events, e)
	}
	return events, rows.Err()
}

// nullableAdminID превращает 0 в NULL — так внешний ключ в схеме
// (ON DELETE SET NULL) работает корректно.
func nullableAdminID(id int64) any {
	if id == 0 {
		return nil
	}
	return id
}
