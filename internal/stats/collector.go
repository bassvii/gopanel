// SPDX-License-Identifier: AGPL-3.0-or-later

// Package stats опрашивает Xray StatsService и пишет приращения
// трафика по пользователям в БД.
package stats

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// CoreState — минимальный интерфейс, чтобы stats не зависел от core.
// Реализуется *xray.Core.
type CoreState interface {
	IsRunning() bool
}

// Collector периодически опрашивает статистику Xray.
type Collector struct {
	db        *sql.DB
	binPath   string
	apiServer string
	log       *slog.Logger
	interval  time.Duration
	core      CoreState

	mu       sync.Mutex
	lastSeen map[string]trafficSample // email → последние значения
}

type trafficSample struct {
	up   int64
	down int64
}

// New создаёт сборщик.
func New(db *sql.DB, binPath, apiServer string, interval time.Duration, core CoreState, log *slog.Logger) *Collector {
	if log == nil {
		log = slog.Default()
	}
	if interval <= 0 {
		interval = 30 * time.Second
	}
	return &Collector{
		db:        db,
		binPath:   binPath,
		apiServer: apiServer,
		log:       log,
		interval:  interval,
		lastSeen:  map[string]trafficSample{},
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
	if c.core != nil && !c.core.IsRunning() {
		c.log.Debug("poll skipped: core not running")
		return
	}
	samples, err := c.queryStats(ctx)
	if err != nil {
		c.log.Warn("stats query failed", "err", err)
		return
	}
	c.log.Debug("stats polled", "count", len(samples))

	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now().Unix()
	for email, sample := range samples {
		last, ok := c.lastSeen[email]

		var deltaUp, deltaDown int64
		switch {
		case !ok:
			// Первый опрос. Если счётчики ненулевые — значит,
			// трафик уже прошёл с момента старта Xray. Записываем его.
			// Если нулевые — просто фиксируем базу.
			deltaUp = sample.up
			deltaDown = sample.down
		case sample.up < last.up || sample.down < last.down:
			// Сброс счётчиков (перезапуск Xray).
			// Считаем, что сброс произошёл в ноль, значит
			// текущее значение — это новое приращение с момента сброса.
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

// queryStats запускает `xray api statsquery` и парсит JSON.
func (c *Collector) queryStats(ctx context.Context) (map[string]trafficSample, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, c.binPath,
		"api", "statsquery",
		"--server="+c.apiServer,
		"-pattern", "user>>>",
	)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("statsquery: %w", err)
	}
	return parseStats(out)
}

// parseStats разбирает JSON-ответ statsquery.
// Формат:
//
//	{"stat":[{"name":"user>>>email>>>traffic>>>uplink","value":123}, ...]}
func parseStats(data []byte) (map[string]trafficSample, error) {
	var resp struct {
		Stat []struct {
			Name  string `json:"name"`
			Value *int64 `json:"value,omitempty"`
		} `json:"stat"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parse stats: %w", err)
	}

	out := map[string]trafficSample{}
	for _, s := range resp.Stat {
		// name: user>>>email>>>traffic>>>uplink
		parts := strings.Split(s.Name, ">>>")
		if len(parts) != 4 || parts[0] != "user" || parts[2] != "traffic" {
			continue
		}
		email := parts[1]
		direction := parts[3]

		var value int64
		if s.Value != nil {
			value = *s.Value
		}

		sample := out[email]
		switch direction {
		case "uplink":
			sample.up = value
		case "downlink":
			sample.down = value
		default:
			continue
		}
		out[email] = sample
	}
	return out, nil
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
		// Пользователь удалён, но Xray ещё помнит его счётчики. Игнорируем.
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
