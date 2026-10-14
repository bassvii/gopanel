// SPDX-License-Identifier: AGPL-3.0-or-later

package api

import (
	"net/http"
	"strconv"

	"github.com/bassvii/gopanel/internal/audit"
	"github.com/bassvii/gopanel/internal/auth"
)

// verifyFreshPassword — общая проверка пароля для sensitive-ручек
// (смена пароля, бэкап, disable 2FA, setup/enable 2FA).
//
//  1. Уважает admin-lockout: если аккаунт залочен — 429, argon2
//     не запускается (не даём перебирать пароль через эти ручки).
//  2. Неверный пароль — RecordAdminFailedAttempt + аудит, 403.
//  3. Верный — ClearAdminFailedAttempts.
//
// Возвращает true, если можно продолжать. При false ответ уже
// отправлен. См. P1-2 третьего аудита.
func (s *Server) verifyFreshPassword(w http.ResponseWriter, r *http.Request, sess auth.Session, admin auth.Admin, password, auditAction string) bool {
	// 1. Lockout.
	locked, wait, err := auth.IsAdminLockedOut(s.db, admin.ID)
	if err != nil {
		s.log.Error("admin lockout check failed", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return false
	}
	if locked {
		w.Header().Set("Retry-After", strconv.Itoa(int(wait.Seconds())))
		writeJSON(w, http.StatusTooManyRequests, map[string]string{
			"error": "too many attempts, try later",
		})
		return false
	}

	// 2. Пароль.
	if err := auth.VerifyPassword(password, admin.PasswordHash); err != nil {
		_ = auth.RecordAdminFailedAttempt(s.db, admin.ID)
		_ = audit.Log(s.db, sess.AdminID, auditAction+"_failed", "password", s.clientIP(r))
		writeJSON(w, http.StatusForbidden, map[string]string{
			"error": "invalid credentials",
		})
		return false
	}
	_ = auth.ClearAdminFailedAttempts(s.db, admin.ID)
	return true
}
