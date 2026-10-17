// SPDX-License-Identifier: AGPL-3.0-or-later

package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/bassvii/gopanel/internal/audit"
	"github.com/bassvii/gopanel/internal/auth"
)

type changePasswordRequest struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

// handleChangePassword — POST /me/password
func (s *Server) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireSession(w, r)
	if !ok {
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.requireCSRF(w, r, sess) {
		return
	}

	var req changePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if len(req.NewPassword) < 12 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "пароль должен быть не короче 12 символов"})
		return
	}

	// Проверяем lockout и пароль через общий хелпер (см. P1-2
	// третьего аудита). Дальше auth.ChangePassword ещё раз
	// проверит старый пароль — это не дублирование, а атомарность
	// в одной транзакции с UPDATE.
	admin, err := auth.GetAdminByID(s.db, sess.AdminID)
	if err != nil {
		s.log.Error("get admin failed", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if !s.verifyFreshPassword(w, r, sess, admin, req.OldPassword, "password_change", true) {
		return
	}

	if err := auth.ChangePassword(s.db, sess.AdminID, sess.ID, req.OldPassword, req.NewPassword); err != nil {
		if errors.Is(err, auth.ErrOldPasswordIncorrect) {
			// Сюда попадаем только если пароль изменился между
			// verifyFreshPassword и ChangePassword — гонка, крайне
			// маловероятная. Считаем как неудачу.
			_ = auth.RecordAdminFailedAttempt(s.db, sess.AdminID)
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "old password is incorrect"})
			return
		}
		s.log.Error("change password failed", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	_ = audit.Log(s.db, sess.AdminID, "password_change", "", s.clientIP(r))
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

type sessionDTO struct {
	ID        string `json:"id"`
	IP        string `json:"ip"`
	UserAgent string `json:"user_agent"`
	LastSeen  int64  `json:"last_seen"`
	ExpiresAt int64  `json:"expires_at"`
	Current   bool   `json:"current"`
}

// handleSessions — GET /me/sessions, DELETE /me/sessions
func (s *Server) handleSessions(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	switch r.Method {
	case http.MethodGet:
		list, err := auth.ListSessions(s.db, sess.AdminID)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		dtos := make([]sessionDTO, 0, len(list))
		for _, x := range list {
			dtos = append(dtos, sessionDTO{
				ID:        x.ID,
				IP:        x.IP,
				UserAgent: x.UserAgent,
				LastSeen:  x.LastSeen.Unix(),
				ExpiresAt: x.ExpiresAt.Unix(),
				Current:   x.ID == sess.ID,
			})
		}
		writeJSON(w, http.StatusOK, map[string]any{"sessions": dtos})

	case http.MethodDelete:
		if !s.requireCSRF(w, r, sess) {
			return
		}
		// Удалить все, кроме текущей.
		if err := auth.DeleteOtherSessions(s.db, sess.AdminID, sess.ID); err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		_ = audit.Log(s.db, sess.AdminID, "sessions_revoked", "", s.clientIP(r))
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleSession — DELETE /me/sessions/{id}
func (s *Server) handleSession(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireSession(w, r)
	if !ok {
		return
	}
	if !s.requireCSRF(w, r, sess) {
		return
	}

	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "invalid session id", http.StatusBadRequest)
		return
	}
	if id == sess.ID {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "нельзя завершить текущую сессию"})
		return
	}

	if err := auth.DeleteSessionForAdmin(s.db, sess.AdminID, id); err != nil {
		if errors.Is(err, auth.ErrSessionNotFound) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	_ = audit.Log(s.db, sess.AdminID, "session_revoked", id, s.clientIP(r))
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
