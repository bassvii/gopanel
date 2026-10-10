// SPDX-License-Identifier: AGPL-3.0-or-later

// Package xray реализует интерфейс core.Core для Xray-core.
package xray

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"log/slog"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

// ErrBinaryNotFound возвращается, если бинарник Xray недоступен.
var ErrBinaryNotFound = errors.New("xray binary not found")

// Core — реализация core.Core для Xray.
type Core struct {
	binPath    string
	configPath string
	log        *slog.Logger
	process    *Process
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
	}, nil
}

// BinPath возвращает путь к бинарнику.
func (c *Core) BinPath() string { return c.binPath }

// ConfigPath возвращает путь к файлу конфига.
func (c *Core) ConfigPath() string { return c.configPath }

var versionRe = regexp.MustCompile(`Xray\s+(\S+)`)

// Version запускает `xray version` и возвращает строку версии.
func (c *Core) Version(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, c.binPath, "version")
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
		return fmt.Errorf("xray -test failed: %w\n%s", err, out)
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
// либо перезапускает Xray. Если проверка не прошла — старый конфиг
// и процесс не трогаются.
func (c *Core) ApplyConfig(ctx context.Context, data []byte) error {
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

	// 3. Если процесс запущен — перезапустить, иначе запустить.
	if c.process.IsRunning() {
		if err := c.process.Stop(); err != nil && !errors.Is(err, ErrNotRunning) {
			return fmt.Errorf("stop for reload: %w", err)
		}
	}
	return c.process.Start(ctx)
}

// tempConfigPath возвращает путь к временному файлу с расширением .json,
// чтобы Xray мог определить формат конфига.
// "/tmp/gopanel-xray.json" → "/tmp/gopanel-xray.tmp.json"
func tempConfigPath(configPath string) string {
	dir := filepath.Dir(configPath)
	base := filepath.Base(configPath)
	ext := filepath.Ext(base)          // ".json"
	name := strings.TrimSuffix(base, ext)  // "gopanel-xray"
	return filepath.Join(dir, name+".tmp"+ext)
}

// Start запускает Xray (конфиг уже должен быть на диске).
func (c *Core) Start(ctx context.Context) error {
	return c.process.Start(ctx)
}

// Stop останавливает Xray.
func (c *Core) Stop() error {
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
