// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build linux

package xray

import "syscall"

// sysProcAttr возвращает SysProcAttr с Pdeathsig на Linux.
//
// Pdeathsig: если родительский процесс (gopanel) умрёт — ядро
// Linux автоматически пошлёт дочернему процессу Xray SIGTERM.
// Это предотвращает «висящий» Xray, который держит порты после
// нештатного падения панели (kill -9, segfault).
//
// См. P2-19 второго аудита.
func sysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		Pdeathsig: syscall.SIGTERM,
	}
}
