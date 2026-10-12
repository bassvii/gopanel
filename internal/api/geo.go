// SPDX-License-Identifier: AGPL-3.0-or-later

package api

import (
	"net/http"
	"strings"

	"github.com/bassvii/gopanel/internal/audit"
	"github.com/bassvii/gopanel/internal/system"
)

// handleGeoStatus — GET /system/geo/status
func (s *Server) handleGeoStatus(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireSession(w, r); !ok {
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, system.GetGeoStatus())
}

// handleGeoUpdate — POST /system/geo/update
func (s *Server) handleGeoUpdate(w http.ResponseWriter, r *http.Request) {
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

	updated, err := system.UpdateGeo(r.Context())
	if err != nil {
		s.log.Error("geo update failed", "err", err)

		// Понятное сообщение для permission denied.
		msg := err.Error()
		if strings.Contains(msg, "permission denied") {
			msg = "нет прав на запись в /usr/local/share/xray — запустите панель с правами root или выдайте права пользователю gopanel"
		}

		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error":   msg,
			"updated": updated,
		})
		return
	}

	_ = audit.Log(s.db, sess.AdminID, "geo_update", "", clientIP(r))

	// Перезапустить Xray, чтобы он подхватил новые файлы.
	s.requestReload()

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"updated": updated,
		"status":  system.GetGeoStatus(),
	})
}
