// SPDX-License-Identifier: AGPL-3.0-or-later

package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/bassvii/gopanel/internal/audit"
	"github.com/bassvii/gopanel/internal/db"
)

type outboundDTO struct {
	ID           int64  `json:"id"`
	Tag          string `json:"tag"`
	Protocol     string `json:"protocol"`
	SettingsJSON string `json:"settings_json"`
	StreamJSON   string `json:"stream_json"`
	Enabled      bool   `json:"enabled"`
	IsBuiltin    bool   `json:"is_builtin"`
	CreatedAt    int64  `json:"created_at"`
}

type outboundPayload struct {
	Tag          string `json:"tag"`
	Protocol     string `json:"protocol"`
	SettingsJSON string `json:"settings_json"`
	StreamJSON   string `json:"stream_json"`
	Enabled      bool   `json:"enabled"`
}

func toOutboundDTO(o db.Outbound) outboundDTO {
	return outboundDTO{
		ID:           o.ID,
		Tag:          o.Tag,
		Protocol:     o.Protocol,
		SettingsJSON: o.SettingsJSON,
		StreamJSON:   o.StreamJSON,
		Enabled:      o.Enabled,
		IsBuiltin:    o.IsBuiltin,
		CreatedAt:    o.CreatedAt.Unix(),
	}
}

// handleOutbounds — GET /outbounds, POST /outbounds
func (s *Server) handleOutbounds(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	switch r.Method {
	case http.MethodGet:
		list, err := db.ListOutbounds(s.db)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		dtos := make([]outboundDTO, 0, len(list))
		for _, o := range list {
			dtos = append(dtos, toOutboundDTO(o))
		}
		writeJSON(w, http.StatusOK, map[string]any{"outbounds": dtos})

	case http.MethodPost:
		if !s.requireCSRF(w, r, sess) {
			return
		}
		var p outboundPayload
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
			return
		}
		if err := validateOutbound(p); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		id, err := db.CreateOutbound(s.db, db.Outbound{
			Tag:          p.Tag,
			Protocol:     p.Protocol,
			SettingsJSON: p.SettingsJSON,
			StreamJSON:   p.StreamJSON,
			Enabled:      p.Enabled,
		})
		if err != nil {
			s.log.Error("create outbound failed", "err", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		_ = audit.Log(s.db, sess.AdminID, "outbound_create", p.Tag, s.clientIP(r))
		s.requestReload()
		writeJSON(w, http.StatusOK, map[string]any{"id": id})

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleOutbound — GET/PUT/DELETE /outbounds/{id}
func (s *Server) handleOutbound(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		o, err := db.GetOutbound(s.db, id)
		if errors.Is(err, db.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, toOutboundDTO(o))

	case http.MethodPut:
		if !s.requireCSRF(w, r, sess) {
			return
		}
		var p outboundPayload
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
			return
		}
		if err := validateOutbound(p); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		err := db.UpdateOutbound(s.db, db.Outbound{
			ID:           id,
			Tag:          p.Tag,
			Protocol:     p.Protocol,
			SettingsJSON: p.SettingsJSON,
			StreamJSON:   p.StreamJSON,
			Enabled:      p.Enabled,
		})
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		_ = audit.Log(s.db, sess.AdminID, "outbound_update", p.Tag, s.clientIP(r))
		s.requestReload()
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})

	case http.MethodDelete:
		if !s.requireCSRF(w, r, sess) {
			return
		}
		if err := db.DeleteOutbound(s.db, id); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		_ = audit.Log(s.db, sess.AdminID, "outbound_delete", idStr, s.clientIP(r))
		s.requestReload()
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func validateOutbound(p outboundPayload) error {
	if strings.TrimSpace(p.Tag) == "" {
		return errors.New("tag is required")
	}
	if strings.TrimSpace(p.Protocol) == "" {
		return errors.New("protocol is required")
	}
	if err := db.ValidateJSON(p.SettingsJSON); err != nil {
		return err
	}
	if err := db.ValidateJSON(p.StreamJSON); err != nil {
		return err
	}
	return nil
}
