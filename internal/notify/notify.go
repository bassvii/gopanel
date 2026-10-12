// SPDX-License-Identifier: AGPL-3.0-or-later

// Package notify создаёт уведомления для админа.
package notify

import (
	"database/sql"
	"fmt"
	"log/slog"

	"github.com/bassvii/gopanel/internal/db"
)

// Категории.
const (
	CategoryAuth   = "auth"
	CategoryXray   = "xray"
	CategoryQuota  = "quota"
	CategoryBackup = "backup"
	CategorySystem = "system"
)

// Severity.
const (
	SeverityInfo    = "info"
	SeverityWarning = "warning"
	SeverityError   = "error"
)

// Notify создаёт уведомление. Ошибки логируются, но не возвращаются —
// уведомление не должно ломать основную операцию.
func Notify(conn *sql.DB, log *slog.Logger, category, severity, title, body string) {
	if _, err := db.CreateNotification(conn, db.Notification{
		Category: category,
		Severity: severity,
		Title:    title,
		Body:     body,
	}); err != nil {
		if log != nil {
			log.Warn("notify failed", "err", err, "title", title)
		}
	}
}

// Готовые события.

func LoginFailed(conn *sql.DB, log *slog.Logger, username, ip string) {
	Notify(conn, log, CategoryAuth, SeverityWarning,
		"Неудачный вход",
		fmt.Sprintf("Логин: %s, IP: %s", username, ip))
}

func LoginSuccess(conn *sql.DB, log *slog.Logger, username, ip string) {
	Notify(conn, log, CategoryAuth, SeverityInfo,
		"Вход в панель",
		fmt.Sprintf("Логин: %s, IP: %s", username, ip))
}

func XrayRestarted(conn *sql.DB, log *slog.Logger, reason string) {
	Notify(conn, log, CategoryXray, SeverityWarning,
		"Xray перезапущен",
		fmt.Sprintf("Причина: %s", reason))
}

func XrayFailed(conn *sql.DB, log *slog.Logger, err string) {
	Notify(conn, log, CategoryXray, SeverityError,
		"Xray не запустился",
		err)
}

func QuotaExceeded(conn *sql.DB, log *slog.Logger, userName string) {
	Notify(conn, log, CategoryQuota, SeverityWarning,
		"Пользователь исчерпал квоту",
		fmt.Sprintf("Пользователь: %s — отключён", userName))
}

func BackupCreated(conn *sql.DB, log *slog.Logger, fileName string) {
	Notify(conn, log, CategoryBackup, SeverityInfo,
		"Бэкап создан",
		fileName)
}

func GeoUpdated(conn *sql.DB, log *slog.Logger, count int) {
	Notify(conn, log, CategorySystem, SeverityInfo,
		"GeoIP/GeoSite обновлены",
		fmt.Sprintf("Файлов: %d", count))
}
