// SPDX-License-Identifier: AGPL-3.0-or-later

package api

import (
	"net/http"
	"strconv"

	"github.com/bassvii/gopanel/internal/audit"
)

type auditEntry struct {
	ID        int64  `json:"id"`
	AdminID   int64  `json:"admin_id"`
	Action    string `json:"action"`
	Target    string `json:"target"`
	IP        string `json:"ip"`
	Timestamp int64  `json:"ts"`
}

// handleAudit — GET /audit?limit=N
func (s *Server) handleAudit(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireSession(w, r); !ok {
		return
	}

	limit := 100
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = n
		}
	}

	events, err := audit.List(s.db, limit)
	if err != nil {
		s.log.Error("audit list failed", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	out := make([]auditEntry, 0, len(events))
	for _, e := range events {
		out = append(out, auditEntry{
			ID:        e.ID,
			AdminID:   e.AdminID,
			Action:    e.Action,
			Target:    e.Target,
			IP:        e.IP,
			Timestamp: e.Timestamp.Unix(),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"entries": out})
}
