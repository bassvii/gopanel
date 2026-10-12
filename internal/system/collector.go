// SPDX-License-Identifier: AGPL-3.0-or-later

package system

import (
	"context"
	"log/slog"
	"time"
)

// Collector периодически собирает метрики и хранит историю.
type Collector struct {
	interval time.Duration
	diskPath string
	history  *History
	log      *slog.Logger
}

func NewCollector(interval time.Duration, diskPath string, historySize int, log *slog.Logger) *Collector {
	if interval <= 0 {
		interval = 5 * time.Second
	}
	if log == nil {
		log = slog.Default()
	}
	return &Collector{
		interval: interval,
		diskPath: diskPath,
		history:  NewHistory(historySize),
		log:      log,
	}
}

// Run блокируется и собирает метрики до отмены ctx.
func (c *Collector) Run(ctx context.Context) {
	// Прогреваем CPU: первый замер заполняет базу, второй даёт процент.
	_, _ = ReadStats(c.diskPath)
	time.Sleep(1 * time.Second)

	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	c.collectOnce()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.collectOnce()
		}
	}
}

func (c *Collector) collectOnce() {
	s, err := ReadStats(c.diskPath)
	if err != nil {
		c.log.Warn("system stats failed", "err", err)
		return
	}
	c.history.Add(Point{
		Timestamp:  s.Timestamp,
		CPUPercent: s.CPUPercent,
		MemPercent: s.MemPercent,
		DiskPercent: s.DiskPercent,
		LoadAvg1:    s.LoadAvg1,
	})
}

// Current возвращает текущие метрики.
func (c *Collector) Current() (Stats, error) {
	return ReadStats(c.diskPath)
}

// History возвращает историю точек.
func (c *Collector) History() []Point {
	return c.history.Snapshot()
}
