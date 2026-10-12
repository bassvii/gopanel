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
// Возвращает список отключённых пользователей.
func EnforceLimits(conn *sql.DB, log *slog.Logger) ([]string, error) {
	now := time.Now().Unix()

	// Сначала выберем тех, кто будет отключён.
	rows, err := conn.Query(`
SELECT name FROM users
WHERE enabled = 1 AND (
    (quota_bytes > 0 AND used_bytes >= quota_bytes)
    OR (expires_at > 0 AND expires_at <= ?)
)`, now)
	if err != nil {
		return nil, err
	}
	var names []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			rows.Close()
			return nil, err
		}
		names = append(names, n)
	}
	rows.Close()

	if len(names) == 0 {
		return nil, nil
	}

	res, err := conn.Exec(`
UPDATE users SET enabled = 0
WHERE enabled = 1 AND (
    (quota_bytes > 0 AND used_bytes >= quota_bytes)
    OR (expires_at > 0 AND expires_at <= ?)
)`, now)
	if err != nil {
		return nil, err
	}
	n, _ := res.RowsAffected()
	if n > 0 {
		log.Info("users disabled by limits", "count", n)
	}
	return names, nil
}

// RunLimitEnforcer периодически применяет лимиты.
// Если были отключения — сигналит через reloadCh и создаёт уведомления.
func RunLimitEnforcer(ctx context.Context, conn *sql.DB, reloadCh chan<- struct{}, interval time.Duration, log *slog.Logger) {
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
			if len(names) > 0 && reloadCh != nil {
				select {
				case reloadCh <- struct{}{}:
				default:
				}
			}
		}
	}
}
