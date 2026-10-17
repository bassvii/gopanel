// SPDX-License-Identifier: AGPL-3.0-or-later

package api

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/bassvii/gopanel/internal/audit"
	"github.com/bassvii/gopanel/internal/notify"
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

	// Geo-файлы могут качаться до 5 минут. Дефолтный WriteTimeout
	// сервера (30с) обрывает ответ. Продлеваем только для этой
	// ручки и только после аутентификации. См. P3.
	if rc := http.NewResponseController(w); rc != nil {
		_ = rc.SetWriteDeadline(time.Now().Add(6 * time.Minute))
	}

	updated, err := system.UpdateGeo(r.Context())
	if err != nil {
		s.log.Error("geo update failed", "err", err)

		// Понятное сообщение для permission denied.
		msg := err.Error()
		if strings.Contains(msg, "permission denied") {
			msg = fmt.Sprintf("нет прав на запись в %s — выдайте права пользователю gopanel", system.GeoDir())
		}

		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error":   msg,
			"updated": updated,
		})
		return
	}

	_ = audit.Log(s.db, sess.AdminID, "geo_update", "", s.clientIP(r))
	notify.GeoUpdated(s.db, s.log, updated)

	// Перезапустить Xray, чтобы он подхватил новые файлы.
	resp := map[string]any{
		"ok":      true,
		"updated": updated,
		"status":  system.GetGeoStatus(),
		"applied": true,
	}
	if err := s.applyNow(); err != nil {
		resp["applied"] = false
		resp["apply_error"] = err.Error()
	}
	writeJSON(w, http.StatusOK, resp)
}
