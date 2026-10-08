// SPDX-License-Identifier: AGPL-3.0-or-later

package stats

import (
	"context"
	"database/sql"
	"log/slog"
	"time"
)

// EnforceLimits отключает пользователей, у которых:
// - used_bytes >= quota_bytes (и quota_bytes > 0)
// - expires_at > 0 и now > expires_at
// Возвращает число отключённых.
func EnforceLimits(conn *sql.DB, log *slog.Logger) (int, error) {
	now := time.Now().Unix()
	res, err := conn.Exec(`
UPDATE users
SET enabled = 0
WHERE enabled = 1 AND (
    (quota_bytes > 0 AND used_bytes >= quota_bytes)
    OR (expires_at > 0 AND expires_at <= ?)
)`, now)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	if n > 0 {
		log.Info("users disabled by limits", "count", n)
	}
	return int(n), nil
}

// RunLimitEnforcer периодически применяет лимиты.
// Если были отключения — сигналит через reloadCh.
func RunLimitEnforcer(ctx context.Context, conn *sql.DB, reloadCh chan<- struct{}, interval time.Duration, log *slog.Logger) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			n, err := EnforceLimits(conn, log)
			if err != nil {
				log.Warn("enforce limits failed", "err", err)
				continue
			}
			if n > 0 && reloadCh != nil {
				select {
				case reloadCh <- struct{}{}:
				default:
				}
			}
		}
	}
}
