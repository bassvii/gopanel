// SPDX-License-Identifier: AGPL-3.0-or-later

package api

import (
	"encoding/json"
	"net/http"
	"strings"

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

	if err := auth.ChangePassword(s.db, sess.AdminID, req.OldPassword, req.NewPassword); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	_ = audit.Log(s.db, sess.AdminID, "password_change", "", clientIP(r))
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
		_ = audit.Log(s.db, sess.AdminID, "sessions_revoked", "", clientIP(r))
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
	if r.Method != http.MethodDelete {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.requireCSRF(w, r, sess) {
		return
	}

	id := strings.TrimPrefix(r.URL.Path, s.secret+"/me/sessions/")
	if id == "" {
		http.Error(w, "invalid session id", http.StatusBadRequest)
		return
	}
	if id == sess.ID {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "нельзя завершить текущую сессию"})
		return
	}

	if err := auth.DeleteSession(s.db, id); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	_ = audit.Log(s.db, sess.AdminID, "session_revoked", id, clientIP(r))
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
