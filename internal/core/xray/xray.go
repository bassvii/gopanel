// SPDX-License-Identifier: AGPL-3.0-or-later

// Package xray реализует интерфейс core.Core для Xray-core.
package xray

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	statsService "github.com/xtls/xray-core/app/stats/command"
)

// ErrBinaryNotFound возвращается, если бинарник Xray недоступен.
var ErrBinaryNotFound = errors.New("xray binary not found")

// Core — реализация core.Core для Xray.
type Core struct {
	binPath    string
	configPath string
	log        *slog.Logger
	process    *Process
	stats      *StatsClient
	applyMu    sync.Mutex
	BeforeStop func(ctx context.Context)
}

// New создаёт Core, проверяя наличие бинарника.
func New(binPath, configPath string, log *slog.Logger) (*Core, error) {
	if binPath == "" {
		return nil, errors.New("xray bin path is empty")
	}
	if _, err := os.Stat(binPath); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrBinaryNotFound, binPath)
	}
	if configPath == "" {
		return nil, errors.New("config path is empty")
	}
	if log == nil {
		log = slog.Default()
	}
	return &Core{
		binPath:    binPath,
		configPath: configPath,
		log:        log,
		process:    NewProcess(binPath, configPath, log),
		stats:      NewStatsClient("127.0.0.1:10085", log),
	}, nil
}

// BinPath возвращает путь к бинарнику.
func (c *Core) BinPath() string { return c.binPath }

// ConfigPath возвращает путь к файлу конфига.
func (c *Core) ConfigPath() string { return c.configPath }

var versionRe = regexp.MustCompile(`Xray\s+(\S+)`)

// Version запускает `xray version` и возвращает строку версии.
func (c *Core) Version() (string, error) {
	// Свой таймаут — не зависим от контекста HTTP-запроса.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, c.binPath, "version")
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("run xray version: %w", err)
	}
	m := versionRe.FindStringSubmatch(string(out))
	if len(m) < 2 {
		return "", fmt.Errorf("cannot parse version from output: %q", string(out))
	}
	return strings.TrimSpace(m[1]), nil
}

// TestConfig проверяет конфиг, не запуская ядро (флаг -test).
func (c *Core) TestConfig(ctx context.Context, configPath string, timeout time.Duration) error {
	if _, err := os.Stat(configPath); err != nil {
		return fmt.Errorf("config not found: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, c.binPath, "run", "-test", "-c", configPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		// В out могут быть фрагменты конфига. Пишем полный
		// вывод в лог, клиенту отдаём усечённый. См. P3.
		c.log.Error("xray -test failed", "err", err, "output", string(out))
		return fmt.Errorf("xray -test failed: %w", err)
	}
	return nil
}

// WriteConfig записывает конфиг в файл (атомарно, с правами 0600).
func (c *Core) WriteConfig(data []byte) error {
	tmp := tempConfigPath(c.configPath)
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write temp config: %w", err)
	}
	if err := os.Rename(tmp, c.configPath); err != nil {
		return fmt.Errorf("rename config: %w", err)
	}
	return nil
}

// ApplyConfig записывает конфиг, проверяет его, и либо запускает,
// либо перезапускает Xray. Если проверка не прошла или процесс
// не стартовал — старый конфиг и процесс остаются нетронутыми
// (или восстанавливаются из .good). См. P0-2, P1-3.
func (c *Core) ApplyConfig(ctx context.Context, data []byte) error {
	c.applyMu.Lock()
	defer c.applyMu.Unlock()

	// 1. Записать во временный файл (с расширением .json, иначе Xray
	// не определит формат) и проверить.
	tmp := tempConfigPath(c.configPath)
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	if err := c.TestConfig(ctx, tmp, 10*time.Second); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("config validation failed: %w", err)
	}

	// 2. Атомарно заменить рабочий файл.
	if err := os.Rename(tmp, c.configPath); err != nil {
		return fmt.Errorf("replace config: %w", err)
	}

	// 3. Если процесс запущен — сначала слить статистику, потом
	// остановить, иначе потеряем трафик за последние 30 секунд.
	if c.process.IsRunning() {
		if c.BeforeStop != nil {
			c.BeforeStop(ctx)
		}
		if err := c.process.Stop(); err != nil && !errors.Is(err, ErrNotRunning) {
			return fmt.Errorf("stop for reload: %w", err)
		}
	}
	if err := c.process.Start(ctx); err != nil {
		// Процесс не запустился: откатываемся на .good.
		return c.rollback(ctx, fmt.Errorf("start new config: %w", err))
	}

	if err := c.waitStarted(1500 * time.Millisecond); err != nil {
		// Процесс упал сразу после старта: откатываемся на .good.
		return c.rollback(ctx, fmt.Errorf("new config failed: %w", err))
	}

	// Всё ок — сохраняем как .good на будущее.
	c.saveGood(data)

	// Xray только что перезапустился — старое gRPC-соединение
	// (если было) порвано. Переподключаемся.
	if err := c.stats.Connect(ctx); err != nil {
		// Не фатально: Xray работает, просто статистика будет
		// недоступна до следующей попытки. Логируем.
		c.log.Warn("stats grpc connect failed", "err", err)
	}
	return nil
}

// goodConfigPath возвращает путь к файлу последнего рабочего конфига.
func (c *Core) goodConfigPath() string {
	return c.configPath + ".good"
}

// saveGood сохраняет текущий конфиг как последний рабочий.
func (c *Core) saveGood(data []byte) {
	if err := os.WriteFile(c.goodConfigPath(), data, 0o600); err != nil {
		c.log.Warn("save good config failed", "err", err)
	}
}

// rollback откатывается на последний рабочий конфиг (если он есть).
// Если откат не удался — возвращает исходную ошибку. Если удался —
// возвращает ошибку с пометкой о восстановлении.
func (c *Core) rollback(ctx context.Context, origErr error) error {
	good, err := os.ReadFile(c.goodConfigPath())
	if err != nil {
		// Нет последнего рабочего конфига — оставляем Xray как есть.
		return origErr
	}

	c.log.Warn("apply failed, rolling back to last good config", "err", origErr)

	// Записываем .good обратно в конфиг.
	tmp := tempConfigPath(c.configPath)
	if err := os.WriteFile(tmp, good, 0o600); err != nil {
		return fmt.Errorf("%w (rollback: write good: %v)", origErr, err)
	}
	if err := c.TestConfig(ctx, tmp, 10*time.Second); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("%w (rollback: test good: %v)", origErr, err)
	}
	if err := os.Rename(tmp, c.configPath); err != nil {
		return fmt.Errorf("%w (rollback: rename: %v)", origErr, err)
	}

	// Перезапускаем процесс на старом конфиге.
	if err := c.process.Start(ctx); err != nil {
		return fmt.Errorf("%w (rollback: start: %v)", origErr, err)
	}
	if err := c.waitStarted(1500 * time.Millisecond); err != nil {
		return fmt.Errorf("%w (rollback: %v)", origErr, err)
	}

	if err := c.stats.Connect(ctx); err != nil {
		c.log.Warn("stats grpc connect failed after rollback", "err", err)
	}
	return fmt.Errorf("%w (восстановлен предыдущий рабочий конфиг)", origErr)
}

// waitStarted ждёт до timeout, проверяя, что процесс жив.
// Если процесс упал за это время — возвращает LastError.
func (c *Core) waitStarted(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !c.process.IsRunning() {
			if err := c.process.LastError(); err != nil {
				return fmt.Errorf("xray failed to start: %w", err)
			}
			return errors.New("xray failed to start (unknown error)")
		}
		time.Sleep(50 * time.Millisecond)
	}
	return nil
}

// tempConfigPath возвращает путь к временному файлу с расширением .json,
// чтобы Xray мог определить формат конфига.
// "/tmp/gopanel-xray.json" → "/tmp/gopanel-xray.tmp.json"
func tempConfigPath(configPath string) string {
	dir := filepath.Dir(configPath)
	base := filepath.Base(configPath)
	ext := filepath.Ext(base)             // ".json"
	name := strings.TrimSuffix(base, ext) // "gopanel-xray"
	return filepath.Join(dir, name+".tmp"+ext)
}

// Start запускает Xray (конфиг уже должен быть на диске).
func (c *Core) Start(ctx context.Context) error {
	return c.process.Start(ctx)
}

// Stop останавливает Xray.
func (c *Core) Stop() error {
	_ = c.stats.Close()
	return c.process.Stop()
}

// IsRunning — жив ли процесс.
func (c *Core) IsRunning() bool {
	return c.process.IsRunning()
}

// LastError — ошибка последнего запуска.
func (c *Core) LastError() error {
	return c.process.LastError()
}

// X25519KeyPair — пара ключей для REALITY.
type X25519KeyPair struct {
	PrivateKey string `json:"private_key"`
	PublicKey  string `json:"public_key"`
}

var (
	x25519PrivateRe = regexp.MustCompile(`(?i)Private\s*Key:\s*(\S+)`)
	x25519PublicRe  = regexp.MustCompile(`(?i)(?:Public\s*Key|Password\s*\(PublicKey\)):\s*(\S+)`)
)

// GenerateX25519 запускает `xray x25519` и возвращает пару ключей.
func (c *Core) GenerateX25519(ctx context.Context) (X25519KeyPair, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, c.binPath, "x25519")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return X25519KeyPair{}, fmt.Errorf("xray x25519 failed: %w\n%s", err, out)
	}

	text := string(out)
	priv := x25519PrivateRe.FindStringSubmatch(text)
	pub := x25519PublicRe.FindStringSubmatch(text)

	if len(priv) < 2 || len(pub) < 2 {
		return X25519KeyPair{}, fmt.Errorf("cannot parse x25519 output: %q", text)
	}

	return X25519KeyPair{
		PrivateKey: priv[1],
		PublicKey:  pub[1],
	}, nil
}

// UptimeSeconds возвращает uptime Xray в секундах.
func (c *Core) UptimeSeconds() int64 {
	return c.process.UptimeSeconds()
}

// StatsClient возвращает gRPC-клиент StatsService.
// Используется сборщиком статистики (internal/stats).
//
// Возвращаем интерфейс, а не *StatsClient, чтобы stats не зависел
// от конкретного типа (и не импортировал core/xray).
func (c *Core) StatsClient() interface {
	QueryStats(ctx context.Context) ([]*statsService.Stat, error)
} {
	return c.stats
}

// VlessEncResult — результат генерации VLESS Encryption.
type VlessEncResult struct {
	Server string `json:"server"`
	Client string `json:"client"`
}

var (
	vlessEncServerRe = regexp.MustCompile(`(?m)^"decryption":\s*"([^"]+)"`)
	vlessEncClientRe = regexp.MustCompile(`(?m)^"encryption":\s*"([^"]+)"`)
)

// GenerateVlessEnc запускает `xray vlessenc` и возвращает первый вариант (X25519).
// Первый вариант — короткие строки, лучше совместимы с клиентами.
// ML-KEM-768 (пост-квантовый) даёт огромные строки — не помещаются в URL.
func (c *Core) GenerateVlessEnc() (VlessEncResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, c.binPath, "vlessenc")
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	out, err := cmd.Output()
	if err != nil {
		return VlessEncResult{}, fmt.Errorf("xray vlessenc failed: %w", err)
	}

	text := string(out)
	server := vlessEncServerRe.FindStringSubmatch(text)
	client := vlessEncClientRe.FindStringSubmatch(text)

	if len(server) < 2 || len(client) < 2 {
		return VlessEncResult{}, fmt.Errorf("cannot parse vlessenc output")
	}

	return VlessEncResult{
		Server: server[1],
		Client: client[1],
	}, nil
}
