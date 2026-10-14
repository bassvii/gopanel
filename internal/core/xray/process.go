// SPDX-License-Identifier: AGPL-3.0-or-later

package xray

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Ошибки процесса.
var (
	ErrNotRunning = errors.New("xray is not running")
	ErrRunning    = errors.New("xray is already running")
)

// Process управляет дочерним процессом Xray.
type Process struct {
	binPath    string
	configPath string
	log        *slog.Logger

	mu       sync.Mutex
	cmd      *exec.Cmd
	cancel   context.CancelFunc
	done     chan struct{}
	lastErr  error
	restarts int

	// Атомики — чтобы читать без мьютекса.
	running   atomic.Bool
	startedAt atomic.Int64 // Unix-время старта, 0 если не запущен
}

// NewProcess создаёт управляющий объект (но не запускает).
func NewProcess(binPath, configPath string, log *slog.Logger) *Process {
	if log == nil {
		log = slog.Default()
	}
	return &Process{
		binPath:    binPath,
		configPath: configPath,
		log:        log,
	}
}

// IsRunning возвращает true, если процесс жив.
// Без мьютекса — читает атомик.
func (p *Process) IsRunning() bool {
	return p.running.Load()
}

// LastError возвращает ошибку последнего запуска (nil, если всё ок).
func (p *Process) LastError() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.lastErr
}

// UptimeSeconds возвращает, сколько работает текущий процесс.
// Без мьютекса — читает атомик.
func (p *Process) UptimeSeconds() int64 {
	start := p.startedAt.Load()
	if start == 0 {
		return 0
	}
	return time.Now().Unix() - start
}

// Start запускает Xray. Если он уже запущен — ErrRunning.
// После успешного старта следит за процессом в фоне и перезапускает
// при падении с экспоненциальной задержкой.
func (p *Process) Start(ctx context.Context) error {
	p.mu.Lock()
	if p.cmd != nil && p.cmd.Process != nil && p.cmd.ProcessState == nil {
		p.mu.Unlock()
		return ErrRunning
	}
	p.mu.Unlock()

	runCtx, cancel := context.WithCancel(ctx)

	cmd := exec.CommandContext(runCtx, p.binPath, "run", "-c", p.configPath)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	// На Linux Pdeathsig=SIGTERM: если панель упадёт нештатно,
	// ядро пошлёт Xray SIGTERM и он сам остановится, освободив
	// порты. На других ОС — nil (без эффекта).
	cmd.SysProcAttr = sysProcAttr()

	if err := cmd.Start(); err != nil {
		cancel()
		return fmt.Errorf("start xray: %w", err)
	}

	p.mu.Lock()
	p.cmd = cmd
	p.cancel = cancel
	p.done = make(chan struct{})
	p.lastErr = nil
	p.mu.Unlock()

	p.running.Store(true)
	p.startedAt.Store(time.Now().Unix())

	p.log.Info("xray started", "pid", cmd.Process.Pid, "config", p.configPath)

	go p.supervise(runCtx, cmd)
	return nil
}

// Stop останавливает Xray и ждёт завершения.
func (p *Process) Stop() error {
	p.mu.Lock()
	if p.cmd == nil || p.cmd.Process == nil {
		p.mu.Unlock()
		return ErrNotRunning
	}
	cancel := p.cancel
	done := p.done
	p.mu.Unlock()

	cancel()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		p.mu.Lock()
		if p.cmd != nil && p.cmd.Process != nil {
			_ = p.cmd.Process.Kill()
		}
		p.mu.Unlock()
		<-done
	}

	p.log.Info("xray stopped")
	return nil
}

// supervise ждёт завершения процесса. Если он упал сам (не по Stop),
// пытается перезапустить с backoff.
func (p *Process) supervise(ctx context.Context, cmd *exec.Cmd) {
	err := cmd.Wait()

	p.mu.Lock()
	p.lastErr = err
	done := p.done
	p.cmd = nil
	p.cancel = nil
	close(done)
	restarts := p.restarts
	uptime := time.Now().Unix() - p.startedAt.Load()
	p.mu.Unlock()

	p.running.Store(false)
	p.startedAt.Store(0)

	if ctx.Err() != nil {
		// Штатная остановка — перезапускать не нужно.
		return
	}

	// Если процесс проработал больше 60 секунд, он был здоровым.
	// Сбрасываем счётчик перезапусков: следующий сбой — не «пятый
	// подряд», а новый инцидент. См. P1-13 второго аудита.
	if uptime > 60 {
		p.mu.Lock()
		p.restarts = 0
		restarts = 0
		p.mu.Unlock()
	}

	// Фатальные ошибки — не перезапускаем.
	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "address already in use") ||
			strings.Contains(msg, "bind:") ||
			strings.Contains(msg, "permission denied") {
			p.log.Error("xray cannot start (fatal error), giving up", "err", err)
			return
		}
	}

	p.log.Warn("xray exited unexpectedly", "err", err, "restarts", restarts)

	// Экспоненциальная задержка: 1s, 2s, 4s, 8s, максимум 30s.
	delay := time.Duration(1<<min(restarts, 5)) * time.Second
	if delay > 30*time.Second {
		delay = 30 * time.Second
	}

	select {
	case <-ctx.Done():
		return
	case <-time.After(delay):
	}

	p.mu.Lock()
	p.restarts++
	p.mu.Unlock()

	if err := p.Start(ctx); err != nil {
		p.log.Error("xray restart failed", "err", err)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
