// SPDX-License-Identifier: AGPL-3.0-or-later

package xray

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	statsService "github.com/xtls/xray-core/app/stats/command"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// StatsClient — обёртка над gRPC StatsServiceClient Xray.
//
// Одно долгоживущее соединение. Переподключается при рестарте Xray
// (см. Connect/Close). Все методы безопасны для конкурентного вызова.
type StatsClient struct {
	addr string
	log  *slog.Logger

	mu   sync.Mutex
	conn *grpc.ClientConn
	cli  statsService.StatsServiceClient
}

// NewStatsClient создаёт объект, но НЕ подключается.
func NewStatsClient(addr string, log *slog.Logger) *StatsClient {
	if log == nil {
		log = slog.Default()
	}
	return &StatsClient{addr: addr, log: log}
}

// Connect устанавливает соединение. Если соединение уже есть — закрывает
// старое и открывает новое.
func (c *StatsClient) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		_ = c.conn.Close()
		c.conn = nil
		c.cli = nil
	}

	// grpc.NewClient (не Dial): официально рекомендован с grpc 1.63+.
	// Ошибка здесь только при неверном целевом адресе — само соединение
	// устанавливается лениво при первом вызове.
	conn, err := grpc.NewClient(c.addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return fmt.Errorf("grpc new client %s: %w", c.addr, err)
	}

	c.conn = conn
	c.cli = statsService.NewStatsServiceClient(conn)
	c.log.Info("stats grpc connected", "addr", c.addr)
	return nil
}

// Close закрывает соединение.
func (c *StatsClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == nil {
		return nil
	}
	err := c.conn.Close()
	c.conn = nil
	c.cli = nil
	return err
}

// QueryStats возвращает все счётчики, начинающиеся с "user>>>".
// reset=false — счётчики не сбрасываются.
func (c *StatsClient) QueryStats(ctx context.Context) ([]*statsService.Stat, error) {
	c.mu.Lock()
	cli := c.cli
	c.mu.Unlock()
	if cli == nil {
		return nil, fmt.Errorf("stats client not connected")
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	resp, err := cli.QueryStats(ctx, &statsService.QueryStatsRequest{
		Pattern: "user>>>",
		Reset_:  false,
	})
	if err != nil {
		return nil, fmt.Errorf("query stats: %w", err)
	}
	return resp.GetStat(), nil
}

// GetOnlineUsers НЕ реализован: в Xray-core v26.3.27 нет
// StatsService.GetUsersStats. Онлайн-IP по пользователю доступны
// через `xray api statsonlineiplist` (exec), но это возвращает
// нас к проблеме fork+exec. Реализуем, когда возьмёмся за лимит
// устройств (PLAN.ru.md, Этап 6): тогда проверим свежие версии
// Xray — возможно, RPC появится.
