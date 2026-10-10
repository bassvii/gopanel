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
	ActionRU  string `json:"action_ru"`
	Target    string `json:"target"`
	IP        string `json:"ip"`
	Timestamp int64  `json:"ts"`
}

// handleAudit — GET /audit?limit=N&offset=M&q=поиск
func (s *Server) handleAudit(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireSession(w, r); !ok {
		return
	}

	q := r.URL.Query()
	limit := 100
	if v := q.Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = n
		}
	}
	offset := 0
	if v := q.Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			offset = n
		}
	}

	events, total, err := audit.List(s.db, audit.ListParams{
		Limit:  limit,
		Offset: offset,
		Query:  q.Get("q"),
	})
	if err != nil {
		s.log.Error("audit list failed", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	out := make([]auditEntry, 0, len(events))
	for _, e := range events {
		ru := e.ActionRU
		if ru == "" {
			ru = e.Action
		}
		out = append(out, auditEntry{
			ID:        e.ID,
			AdminID:   e.AdminID,
			Action:    e.Action,
			ActionRU:  ru,
			Target:    e.Target,
			IP:        e.IP,
			Timestamp: e.Timestamp.Unix(),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"entries": out,
		"total":   total,
		"limit":   limit,
		"offset":  offset,
	})
}
