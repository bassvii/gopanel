// SPDX-License-Identifier: AGPL-3.0-or-later

package db

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"
)

// Retention-политики по умолчанию.
const (
	TrafficRetentionDays = 90
	AuditRetentionDays   = 365
)

// CleanupOldRecords удаляет старые записи из traffic и audit_log.
// Идемпотентно: повторный вызов ничего не сломает. Ошибки отдельных
// таблиц логируются, но не блокируют остальные.
func CleanupOldRecords(conn *sql.DB, log *slog.Logger) {
	now := time.Now().Unix()

	trafficCutoff := now - int64(TrafficRetentionDays)*24*3600
	res, err := conn.Exec(`DELETE FROM traffic WHERE ts < ?`, trafficCutoff)
	if err != nil {
		log.Warn("traffic retention failed", "err", err)
	} else if n, _ := res.RowsAffected(); n > 0 {
		log.Info("traffic retention", "deleted", n, "older_than_days", TrafficRetentionDays)
	}

	auditCutoff := now - int64(AuditRetentionDays)*24*3600
	res, err = conn.Exec(`DELETE FROM audit_log WHERE ts < ?`, auditCutoff)
	if err != nil {
		log.Warn("audit retention failed", "err", err)
	} else if n, _ := res.RowsAffected(); n > 0 {
		log.Info("audit retention", "deleted", n, "older_than_days", AuditRetentionDays)
	}
}

// RunRetentionLoop запускает очистку раз в сутки до отмены ctx.
// Первая очистка — сразу при старте (на случай, если панель
// не перезапускалась неделями).
func RunRetentionLoop(ctx context.Context, conn *sql.DB, log *slog.Logger) {
	CleanupOldRecords(conn, log)

	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			CleanupOldRecords(conn, log)
		}
	}
}

// RetentionStats возвращает число строк в traffic и audit_log —
// для API/диагностики.
func RetentionStats(conn *sql.DB) (trafficRows, auditRows int64, err error) {
	if err := conn.QueryRow(`SELECT COUNT(*) FROM traffic`).Scan(&trafficRows); err != nil {
		return 0, 0, fmt.Errorf("count traffic: %w", err)
	}
	if err := conn.QueryRow(`SELECT COUNT(*) FROM audit_log`).Scan(&auditRows); err != nil {
		return 0, 0, fmt.Errorf("count audit: %w", err)
	}
	return trafficRows, auditRows, nil
}
