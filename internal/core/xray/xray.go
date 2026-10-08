// SPDX-License-Identifier: AGPL-3.0-or-later

// Package xray реализует интерфейс core.Core для Xray-core.
package xray

import (
	"context"
	"errors"
	"fmt"
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
	binPath string
}

// New создаёт Core, проверяя наличие бинарника.
func New(binPath string) (*Core, error) {
	if binPath == "" {
		return nil, errors.New("xray bin path is empty")
	}
	if _, err := os.Stat(binPath); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrBinaryNotFound, binPath)
	}
	return &Core{binPath: binPath}, nil
}

// BinPath возвращает путь к бинарнику.
func (c *Core) BinPath() string { return c.binPath }

var versionRe = regexp.MustCompile(`Xray\s+(\S+)`)

// Version запускает `xray version` и возвращает строку версии.
func (c *Core) Version(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, c.binPath, "version")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("run xray version: %w", err)
	}
	// Первая строка выглядит так:
	//   Xray 26.3.27 (Xray, Penetrates Everything.) d2758a0 (go1.26.1 linux/amd64)
	m := versionRe.FindStringSubmatch(string(out))
	if len(m) < 2 {
		return "", fmt.Errorf("cannot parse version from output: %q", string(out))
	}
	return strings.TrimSpace(m[1]), nil
}

// TestConfig проверяет конфиг, не запуская ядро (флаг -test).
// timeout — максимальное время проверки.
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
