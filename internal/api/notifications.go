// SPDX-License-Identifier: AGPL-3.0-or-later

package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/bassvii/gopanel/internal/db"
)

type notificationDTO struct {
	ID        int64  `json:"id"`
	Category  string `json:"category"`
	Severity  string `json:"severity"`
	Title     string `json:"title"`
	Body      string `json:"body"`
	ReadAt    int64  `json:"read_at"`
	CreatedAt int64  `json:"created_at"`
}

func toNotificationDTO(n db.Notification) notificationDTO {
	var readAt int64
	if !n.ReadAt.IsZero() {
		readAt = n.ReadAt.Unix()
	}
	return notificationDTO{
		ID:        n.ID,
		Category:  n.Category,
		Severity:  n.Severity,
		Title:     n.Title,
		Body:      n.Body,
		ReadAt:    readAt,
		CreatedAt: n.CreatedAt.Unix(),
	}
}

// handleNotifications — GET /notifications?limit=N
func (s *Server) handleNotifications(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireSession(w, r); !ok {
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = n
		}
	}

	list, err := db.ListNotifications(s.db, limit)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	unread, err := db.UnreadCount(s.db)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	dtos := make([]notificationDTO, 0, len(list))
	for _, n := range list {
		dtos = append(dtos, toNotificationDTO(n))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"notifications": dtos,
		"unread":        unread,
	})
}

// handleNotificationsRead — POST /notifications/read
// Body: {"id": 5} — пометить одно; {"all": true} — пометить все.
func (s *Server) handleNotificationsRead(w http.ResponseWriter, r *http.Request) {
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

	var body struct {
		ID  int64 `json:"id"`
		All bool  `json:"all"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	if body.All {
		if err := db.MarkAllRead(s.db); err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
	} else if body.ID > 0 {
		if err := db.MarkRead(s.db, body.ID); err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
	}

	unread, _ := db.UnreadCount(s.db)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "unread": unread})
}
