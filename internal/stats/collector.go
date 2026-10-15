// SPDX-License-Identifier: AGPL-3.0-or-later

// Package stats опрашивает Xray StatsService через gRPC и пишет
// приращения трафика по пользователям в БД.
package stats

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	statsService "github.com/xtls/xray-core/app/stats/command"
)

// ErrNotConnected — возвращается, если gRPC-клиент ещё не подключён.
// Сборщик использует это, чтобы не спамить Warning при старте
// (до первого ApplyConfig соединение не установлено).
var ErrNotConnected = errors.New("stats client not connected")

// CoreState — минимальный интерфейс, чтобы stats не зависел от core.
// Реализуется *xray.Core.
type CoreState interface {
	IsRunning() bool
	// Возвращает анонимный интерфейс — совпадает по структуре с
	// xray.Core.StatsClient(). Так избегаем общего именованного типа,
	// который тянул бы импорт core/xray в stats.
	StatsClient() interface {
		QueryStats(ctx context.Context) ([]*statsService.Stat, error)
	}
}

// Collector периодически опрашивает статистику Xray.
type Collector struct {
	db       *sql.DB
	log      *slog.Logger
	interval time.Duration
	core     CoreState

	mu       sync.Mutex
	lastSeen map[string]trafficSample // email → последние значения
}

type trafficSample struct {
	up   int64
	down int64
}

// New создаёт сборщик. Параметры binPath и apiServer больше не нужны —
// статистика идёт через gRPC, а не через exec. Оставлены в сигнатуре
// для совместимости с main.go, но игнорируются.
func New(db *sql.DB, _ /*binPath*/, _ /*apiServer*/ string, interval time.Duration, core CoreState, log *slog.Logger) *Collector {
	if log == nil {
		log = slog.Default()
	}
	if interval <= 0 {
		interval = 30 * time.Second
	}
	return &Collector{
		db:       db,
		log:      log,
		interval: interval,
		core:     core,
		lastSeen: map[string]trafficSample{},
	}
}

// Run блокируется, опрашивая статистику до отмены ctx.
func (c *Collector) Run(ctx context.Context) {
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	// Первый опрос — сразу.
	c.pollOnce(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.pollOnce(ctx)
		}
	}
}

// pollOnce делает один опрос.
func (c *Collector) pollOnce(ctx context.Context) {
	if c.core == nil || !c.core.IsRunning() {
		c.log.Debug("poll skipped: core not running")
		return
	}

	client := c.core.StatsClient()
	if client == nil {
		c.log.Debug("poll skipped: stats client not ready")
		return
	}

	stats, err := client.QueryStats(ctx)
	if err != nil {
		c.log.Warn("stats query failed", "err", err)
		return
	}
	samples := parseStats(stats)
	c.log.Debug("stats polled", "count", len(samples))

	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now().Unix()
	for email, sample := range samples {
		last, ok := c.lastSeen[email]

		var deltaUp, deltaDown int64
		switch {
		case !ok:
			deltaUp = sample.up
			deltaDown = sample.down
		case sample.up < last.up || sample.down < last.down:
			// Сброс счётчиков (перезапуск Xray).
			deltaUp = sample.up
			deltaDown = sample.down
		default:
			deltaUp = sample.up - last.up
			deltaDown = sample.down - last.down
		}

		c.lastSeen[email] = sample

		if deltaUp == 0 && deltaDown == 0 {
			continue
		}
		if err := c.recordTraffic(ctx, email, now, deltaUp, deltaDown); err != nil {
			c.log.Warn("record traffic failed", "email", email, "err", err)
		}
	}
}

// parseStats разбирает []*statsService.Stat в map[email]trafficSample.
// Имена: user>>>email>>>traffic>>>uplink/downlink.
func parseStats(stats []*statsService.Stat) map[string]trafficSample {
	out := map[string]trafficSample{}
	for _, s := range stats {
		if s == nil {
			continue
		}
		parts := strings.Split(s.GetName(), ">>>")
		if len(parts) != 4 || parts[0] != "user" || parts[2] != "traffic" {
			continue
		}
		email := parts[1]
		direction := parts[3]

		sample := out[email]
		switch direction {
		case "uplink":
			sample.up = s.GetValue()
		case "downlink":
			sample.down = s.GetValue()
		default:
			continue
		}
		out[email] = sample
	}
	return out
}

// recordTraffic пишет приращение в таблицу traffic и увеличивает
// users.used_bytes.
func (c *Collector) recordTraffic(ctx context.Context, email string, ts, up, down int64) error {
	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var userID int64
	err = tx.QueryRowContext(ctx, `SELECT id FROM users WHERE email = ?`, email).Scan(&userID)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return fmt.Errorf("find user %q: %w", email, err)
	}

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO traffic (user_id, ts, up_bytes, down_bytes) VALUES (?, ?, ?, ?)`,
		userID, ts, up, down,
	); err != nil {
		return fmt.Errorf("insert traffic: %w", err)
	}

	if _, err := tx.ExecContext(ctx,
		`UPDATE users SET used_bytes = used_bytes + ? WHERE id = ?`,
		up+down, userID,
	); err != nil {
		return fmt.Errorf("update used_bytes: %w", err)
	}

	return tx.Commit()
}

// Stats возвращает текущее состояние счётчиков (для отладки/API).
func (c *Collector) Stats() map[string]trafficSample {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make(map[string]trafficSample, len(c.lastSeen))
	for k, v := range c.lastSeen {
		out[k] = v
	}
	return out
}
