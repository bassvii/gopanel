// SPDX-License-Identifier: AGPL-3.0-or-later

package api

import (
	"net/http"
)

type statsSummary struct {
	UsersTotal    int   `json:"users_total"`
	UsersEnabled  int   `json:"users_enabled"`
	InboundsTotal int   `json:"inbounds_total"`
	InboundsOn    int   `json:"inbounds_enabled"`
	TrafficTotal  int64 `json:"traffic_total_bytes"`
	TrafficUsed   int64 `json:"traffic_used_bytes"`
}

// handleStatsSummary — GET /stats/summary
func (s *Server) handleStatsSummary(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireSession(w, r); !ok {
		return
	}

	var out statsSummary

	// Пользователи
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&out.UsersTotal); err != nil {
		s.log.Error("count users failed", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM users WHERE enabled = 1`).Scan(&out.UsersEnabled); err != nil {
		s.log.Error("count enabled users failed", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Инбаунды
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM inbounds`).Scan(&out.InboundsTotal); err != nil {
		s.log.Error("count inbounds failed", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM inbounds WHERE enabled = 1`).Scan(&out.InboundsOn); err != nil {
		s.log.Error("count enabled inbounds failed", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Трафик: сумма used_bytes по всем пользователям.
	if err := s.db.QueryRow(`SELECT COALESCE(SUM(used_bytes), 0) FROM users`).Scan(&out.TrafficUsed); err != nil {
		s.log.Error("sum used_bytes failed", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Общая квота (сумма quota_bytes там, где quota > 0).
	if err := s.db.QueryRow(`SELECT COALESCE(SUM(quota_bytes), 0) FROM users WHERE quota_bytes > 0`).Scan(&out.TrafficTotal); err != nil {
		s.log.Error("sum quota_bytes failed", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, out)
}
