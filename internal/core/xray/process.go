// SPDX-License-Identifier: AGPL-3.0-or-later

package xray

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// tailWriter хранит последние N байт вывода. Нужен, чтобы поймать
// текст ошибки Xray («address already in use», «permission denied»)
// из stderr — cmd.Wait() возвращает только «exit status 1», а сам
// текст уходит в поток.
type tailWriter struct {
	mu  sync.Mutex
	buf []byte
	max int
}

func newTailWriter(max int) *tailWriter {
	return &tailWriter{max: max}
}

func (t *tailWriter) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.buf = append(t.buf, p...)
	if len(t.buf) > t.max {
		t.buf = t.buf[len(t.buf)-t.max:]
	}
	return len(p), nil
}

func (t *tailWriter) String() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return string(t.buf)
}

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
	tw       *tailWriter
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
	tw := newTailWriter(4096)
	p.mu.Lock()
	p.tw = tw
	p.mu.Unlock()
	cmd.Stdout = io.MultiWriter(os.Stderr, tw)
	cmd.Stderr = io.MultiWriter(os.Stderr, tw)
	cmd.SysProcAttr = sysProcAttr()
	// Окружение без секретов панели: GOPANEL_ADMIN_PASSWORD
	// не должен попадать в процесс ядра. См. #10 нового аудита.
	cmd.Env = append(scrubbedEnv(),
		"XRAY_LOCATION_ASSET="+xrayAssetDir(),
	)

	// Pdeathsig в Go привязан к потоку ОС, который вызвал Start(),
	// а не к процессу. Если этот поток завершится (Go-рантайм
	// переиспользует потоки), Xray получит SIGTERM раньше времени.
	// Известный баг: golang/go#27505. См. #10 нового аудита.
	//
	// Решение: запускаем cmd.Start() из выделенной горутины,
	// жёстко привязанной к потоку через runtime.LockOSThread().
	// Горутина живёт столько же, сколько Xray (ждёт Wait()).
	startErr := make(chan error, 1)
	done := make(chan struct{})

	p.mu.Lock()
	p.cmd = cmd
	p.cancel = cancel
	p.done = done
	p.lastErr = nil
	p.mu.Unlock()

	go func() {
		runtime.LockOSThread()
		// Поток не вернётся в пул, пока жива эта горутина.
		// Горутина живёт, пока жив Xray: Wait() блокируется.
		defer runtime.UnlockOSThread()

		if err := cmd.Start(); err != nil {
			startErr <- err
			return
		}
		startErr <- nil

		// Здесь же ждём завершения процесса. Поток остаётся
		// привязанным до конца — Pdeathsig срабатывает корректно.
		p.superviseAfterStart(runCtx, cmd, done)
	}()

	if err := <-startErr; err != nil {
		cancel()
		return fmt.Errorf("start xray: %w", err)
	}

	p.running.Store(true)
	p.startedAt.Store(time.Now().Unix())

	p.log.Info("xray started", "pid", cmd.Process.Pid, "config", p.configPath)

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
		// Ждём ещё немного — Kill() должен сработать.
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			p.log.Warn("xray did not exit after SIGKILL")
		}
	}

	p.log.Info("xray stopped")
	return nil
}

// superviseAfterStart ждёт завершения процесса, вызванного из
// выделенной горутины с LockOSThread. done — канал из Start(),
// который закрывается при выходе (чтобы Stop() знал, что процесс
// завершён).
func (p *Process) superviseAfterStart(ctx context.Context, cmd *exec.Cmd, done chan struct{}) {
	err := cmd.Wait()

	p.mu.Lock()
	p.lastErr = err
	p.cmd = nil
	p.cancel = nil
	close(done)
	restarts := p.restarts
	uptime := time.Now().Unix() - p.startedAt.Load()
	var tail string
	if p.tw != nil {
		tail = p.tw.String()
	}
	p.tw = nil
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

	// Фатальные ошибки — ищем в хвосте stderr, а не в err.Error():
	// cmd.Wait() возвращает только «exit status 1», а текст
	// («address already in use», «permission denied») идёт в поток.
	// См. #10 нового аудита.
	if err != nil {
		if strings.Contains(tail, "address already in use") ||
			strings.Contains(tail, "bind:") ||
			strings.Contains(tail, "permission denied") {
			p.log.Error("xray cannot start (fatal error), giving up",
				"err", err, "stderr_tail", tail)
			return
		}
	}

	// Ограничиваем число перезапусков подряд: 10 попыток и сдаёмся.
	// Иначе при системной проблеме (например, порт занят другим
	// процессом, но текст ошибки не попал в tail) Xray будет
	// перезапускаться бесконечно. См. #10 нового аудита.
	const maxRestarts = 10
	if restarts >= maxRestarts {
		p.log.Error("xray crashed too many times, giving up",
			"restarts", restarts)
		return
	}

	p.log.Warn("xray exited unexpectedly", "err", err, "restarts", restarts)

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

// xrayAssetDir возвращает каталог, в котором Xray ищет geosite.dat
// и geoip.dat. Берётся из GOPANEL_GEO_DIR, если задан, иначе —
// <каталог базы>/xray. См. P2-1 третьего аудита.
//
// Не импортируем system, чтобы не создавать цикл: xray → system,
// а system не должен зависеть от xray.
func xrayAssetDir() string {
	if v := os.Getenv("GOPANEL_GEO_DIR"); v != "" {
		return v
	}
	// Резервный вариант. Если dbPath не известен на этом уровне,
	// используем тот же дефолт, что и system.DefaultGeoDir.
	return "/var/lib/gopanel/xray"
}

// scrubbedEnv возвращает os.Environ() без переменных панели,
// которые не должны утекать в дочерний процесс Xray.
// Сейчас это GOPANEL_ADMIN_PASSWORD.
func scrubbedEnv() []string {
	env := os.Environ()
	out := make([]string, 0, len(env))
	for _, e := range env {
		if strings.HasPrefix(e, "GOPANEL_ADMIN_PASSWORD=") {
			continue
		}
		out = append(out, e)
	}
	return out
}
