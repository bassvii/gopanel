// SPDX-License-Identifier: AGPL-3.0-or-later

package stats

import (
	"context"
	"database/sql"
	"log/slog"
	"time"

	"github.com/bassvii/gopanel/internal/notify"
)

// EnforceLimits отключает пользователей, у которых:
// - used_bytes >= quota_bytes (и quota_bytes > 0)
// - expires_at > 0 и now > expires_at
// Возвращает список имён отключённых пользователей.
//
// Один атомарный UPDATE ... RETURNING — нет окна между SELECT и
// UPDATE, где два конкурентных вызова могли бы сделать одно и то же.
// SQLite 3.35+ (modernc.org/sqlite v1.60) поддерживает RETURNING.
// См. P2-18 второго аудита.
func EnforceLimits(conn *sql.DB, log *slog.Logger) ([]string, error) {
	now := time.Now().Unix()

	rows, err := conn.Query(`
UPDATE users SET enabled = 0
WHERE enabled = 1 AND (
    (quota_bytes > 0 AND used_bytes >= quota_bytes)
    OR (expires_at > 0 AND expires_at <= ?)
)
RETURNING name`, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		names = append(names, n)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(names) > 0 {
		log.Info("users disabled by limits", "count", len(names))
	}
	return names, nil
}

// RunLimitEnforcer периодически применяет лимиты.
// Если были отключения — вызывает applyFn (синхронно) и создаёт уведомления.
// Ошибка применения логируется, но не прерывает цикл.
func RunLimitEnforcer(ctx context.Context, conn *sql.DB, applyFn func() error, interval time.Duration, log *slog.Logger) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			names, err := EnforceLimits(conn, log)
			if err != nil {
				log.Warn("enforce limits failed", "err", err)
				continue
			}
			for _, name := range names {
				notify.QuotaExceeded(conn, log, name)
			}
			if len(names) > 0 && applyFn != nil {
				if err := applyFn(); err != nil {
					log.Error("apply config after limit enforcement failed", "err", err)
					notify.XrayFailed(conn, log, err.Error())
				}
			}
		}
	}
}
