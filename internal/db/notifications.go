// SPDX-License-Identifier: AGPL-3.0-or-later

package db

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Notification — уведомление.
type Notification struct {
	ID        int64
	Category  string
	Severity  string
	Title     string
	Body      string
	ReadAt    time.Time
	CreatedAt time.Time
}

// CreateNotification создаёт уведомление.
func CreateNotification(conn *sql.DB, n Notification) (int64, error) {
	if n.Title == "" {
		return 0, errors.New("title is required")
	}
	if n.Category == "" {
		n.Category = "system"
	}
	if n.Severity == "" {
		n.Severity = "info"
	}

	res, err := conn.Exec(
		`INSERT INTO notifications (category, severity, title, body, read_at, created_at)
		 VALUES (?, ?, ?, ?, 0, ?)`,
		n.Category, n.Severity, n.Title, n.Body, time.Now().Unix(),
	)
	if err != nil {
		return 0, fmt.Errorf("insert notification: %w", err)
	}
	return res.LastInsertId()
}

// ListNotifications возвращает последние N уведомлений.
func ListNotifications(conn *sql.DB, limit int) ([]Notification, error) {
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	rows, err := conn.Query(
		`SELECT id, category, severity, title, body, read_at, created_at
		 FROM notifications ORDER BY created_at DESC LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list notifications: %w", err)
	}
	defer rows.Close()

	var out []Notification
	for rows.Next() {
		var (
			n         Notification
			readAt    int64
			createdAt int64
		)
		if err := rows.Scan(&n.ID, &n.Category, &n.Severity, &n.Title, &n.Body,
			&readAt, &createdAt); err != nil {
			return nil, err
		}
		if readAt > 0 {
			n.ReadAt = time.Unix(readAt, 0)
		}
		n.CreatedAt = time.Unix(createdAt, 0)
		out = append(out, n)
	}
	return out, rows.Err()
}

// UnreadCount возвращает число непрочитанных.
func UnreadCount(conn *sql.DB) (int, error) {
	var n int
	err := conn.QueryRow(`SELECT COUNT(*) FROM notifications WHERE read_at = 0`).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count unread: %w", err)
	}
	return n, nil
}

// MarkRead помечает уведомление прочитанным по id.
func MarkRead(conn *sql.DB, id int64) error {
	_, err := conn.Exec(
		`UPDATE notifications SET read_at = ? WHERE id = ? AND read_at = 0`,
		time.Now().Unix(), id,
	)
	return err
}

// MarkAllRead помечает все уведомления прочитанными.
func MarkAllRead(conn *sql.DB) error {
	_, err := conn.Exec(
		`UPDATE notifications SET read_at = ? WHERE read_at = 0`,
		time.Now().Unix(),
	)
	return err
}

// DeleteOldNotifications удаляет уведомления старше N дней.
func DeleteOldNotifications(conn *sql.DB, days int) (int64, error) {
	if days <= 0 {
		days = 30
	}
	threshold := time.Now().Add(-time.Duration(days) * 24 * time.Hour).Unix()
	res, err := conn.Exec(`DELETE FROM notifications WHERE created_at < ?`, threshold)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
