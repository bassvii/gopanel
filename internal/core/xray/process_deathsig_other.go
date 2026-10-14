// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build !linux

package xray

import "syscall"

// sysProcAttr на не-Linux возвращает nil: Pdeathsig доступен
// только на Linux. На macOS/Windows эта защита не работает —
// в systemd/KillMode=control-group проблема не возникает, но
// при ручном запуске панели Xray может остаться.
//
// См. P2-19 второго аудита.
func sysProcAttr() *syscall.SysProcAttr {
	return nil
}
