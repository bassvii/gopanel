// SPDX-License-Identifier: AGPL-3.0-or-later

// Package audit записывает действия администраторов.
package audit

import (
	"database/sql"
	"fmt"
	"time"
	"strings"
)

// Event — одно действие в журнале.
type Event struct {
	ID        int64
	AdminID   int64
	Action    string
	ActionRU  string
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

// Labels — русские названия для действий.
var Labels = map[string]string{
	"login":               "Вход в панель",
	"logout":              "Выход из панели",
	"login_failed":        "Неудачный вход",
	"totp_enabled":        "Включена 2FA",
	"totp_disabled":       "Отключена 2FA",
	"recovery_code_used":  "Использован код восстановления",
	"password_reset":      "Сброс пароля",
	"password_change":     "Пароль изменён",
	"sessions_revoked":    "Завершены все сессии, кроме текущей",
	"session_revoked":     "Сессия завершена",
	"user_create":         "Создан пользователь",
	"user_update":         "Изменён пользователь",
	"user_delete":         "Удалён пользователь",
	"user_inbound_attach": "Привязан инбаунд к пользователю",
	"user_inbound_detach": "Отвязан инбаунд от пользователя",
	"inbound_create":      "Создан инбаунд",
	"inbound_update":      "Изменён инбаунд",
	"inbound_delete":      "Удалён инбаунд",
}

// label возвращает русское название действия в нижнем регистре.
func label(action string) string {
	if v, ok := Labels[action]; ok {
		return strings.ToLower(v)
	}
	return strings.ToLower(action)
}

// Log записывает событие. adminID == 0 означает «без администратора»
// (например, неудачный вход по несуществующему логину).
func Log(conn *sql.DB, adminID int64, action, target, ip string) error {
	_, err := conn.Exec(
		`INSERT INTO audit_log (admin_id, action, action_ru, target, ip, ts)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		nullableAdminID(adminID), action, label(action), target, ip, time.Now().Unix(),
	)
	if err != nil {
		return fmt.Errorf("write audit log: %w", err)
	}
	return nil
}

// List возвращает последние N записей журнала (по убыванию времени).
// ListParams — параметры фильтрации.
type ListParams struct {
	Limit   int
	Offset  int
	Query   string  // поиск по action, action_ru, target, ip
	AdminID int64
}

// List возвращает записи журнала с фильтрами.
// List возвращает записи журнала с поиском и пагинацией.
func List(conn *sql.DB, p ListParams) ([]Event, int, error) {
	if p.Limit <= 0 || p.Limit > 500 {
		p.Limit = 100
	}

	var (
		where []string
		args  []any
	)
	if p.Query != "" {
		q := "%" + strings.ToLower(p.Query) + "%"
		where = append(where,
			"(action LIKE ? OR action_ru LIKE ? OR target LIKE ? OR ip LIKE ?)")
		args = append(args, q, q, q, q)
	}
	if p.AdminID > 0 {
		where = append(where, "admin_id = ?")
		args = append(args, p.AdminID)
	}

	whereSQL := ""
	if len(where) > 0 {
		whereSQL = "WHERE " + strings.Join(where, " AND ")
	}

	var total int
	countSQL := "SELECT COUNT(*) FROM audit_log " + whereSQL
	if err := conn.QueryRow(countSQL, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count audit log: %w", err)
	}

	query := `SELECT id, COALESCE(admin_id, 0), action, action_ru, target, ip, ts
	          FROM audit_log ` + whereSQL + `
	          ORDER BY ts DESC LIMIT ? OFFSET ?`
	args = append(args, p.Limit, p.Offset)

	rows, err := conn.Query(query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list audit log: %w", err)
	}
	defer rows.Close()

	var events []Event
	for rows.Next() {
		var (
			e  Event
			ts int64
		)
		if err := rows.Scan(&e.ID, &e.AdminID, &e.Action, &e.ActionRU, &e.Target, &e.IP, &ts); err != nil {
			return nil, 0, fmt.Errorf("scan audit log: %w", err)
		}
		e.Timestamp = time.Unix(ts, 0)
		events = append(events, e)
	}
	return events, total, rows.Err()
}

// nullableAdminID превращает 0 в NULL — так внешний ключ в схеме
// (ON DELETE SET NULL) работает корректно.
func nullableAdminID(id int64) any {
	if id == 0 {
		return nil
	}
	return id
}
