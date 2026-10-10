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

type inboundDTO struct {
	ID           int64  `json:"id"`
	Tag          string `json:"tag"`
	Protocol     string `json:"protocol"`
	Port         int    `json:"port"`
	Listen       string `json:"listen"`
	SettingsJSON string `json:"settings_json"`
	StreamJSON   string `json:"stream_json"`
	SniffingJSON string `json:"sniffing_json"`
	Enabled      bool   `json:"enabled"`
	CreatedAt    int64  `json:"created_at"`
}

func toInboundDTO(in db.Inbound) inboundDTO {
	var created int64
	if !in.CreatedAt.IsZero() {
		created = in.CreatedAt.Unix()
	}
	return inboundDTO{
		ID:           in.ID,
		Tag:          in.Tag,
		Protocol:     in.Protocol,
		Port:         in.Port,
		Listen:       in.Listen,
		SettingsJSON: in.SettingsJSON,
		StreamJSON:   in.StreamJSON,
		SniffingJSON: in.SniffingJSON,
		Enabled:      in.Enabled,
		CreatedAt:    created,
	}
}

type inboundPayload struct {
	Tag          string `json:"tag"`
	Protocol     string `json:"protocol"`
	Port         int    `json:"port"`
	Listen       string `json:"listen"`
	SettingsJSON string `json:"settings_json"`
	StreamJSON   string `json:"stream_json"`
	SniffingJSON string `json:"sniffing_json"`
	Enabled      bool   `json:"enabled"`
}

// handleInbounds — GET /inbounds (список), POST /inbounds (создать).
func (s *Server) handleInbounds(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	switch r.Method {
		case http.MethodGet:
			list, err := db.ListInbounds(s.db)
			if err != nil {
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
			dtos := make([]inboundDTO, 0, len(list))
			for _, in := range list {
				dtos = append(dtos, toInboundDTO(in))
			}
			writeJSON(w, http.StatusOK, map[string]any{"inbounds": dtos})

	case http.MethodPost:
		if !s.requireCSRF(w, r, sess) {
			return
		}
		var p inboundPayload
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
			return
		}
		if err := validateInbound(p); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}

		id, err := db.CreateInbound(s.db, db.Inbound{
			Tag: p.Tag, Protocol: p.Protocol, Port: p.Port, Listen: p.Listen,
			SettingsJSON: p.SettingsJSON, StreamJSON: p.StreamJSON,
			SniffingJSON: p.SniffingJSON, Enabled: p.Enabled,
		})
		if err != nil {
			s.log.Error("create inbound failed", "err", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		_ = audit.Log(s.db, sess.AdminID, "inbound_create", p.Tag, clientIP(r))
		s.requestReload()
		writeJSON(w, http.StatusOK, map[string]any{"id": id})

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleInbound — GET/PUT/DELETE /inbounds/{id}
func (s *Server) handleInbound(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	idStr := strings.TrimPrefix(r.URL.Path, s.secret+"/inbounds/")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	switch r.Method {
		case http.MethodGet:
			in, err := db.GetInbound(s.db, id)
			if errors.Is(err, db.ErrNotFound) {
				http.NotFound(w, r)
				return
			}
			if err != nil {
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
			writeJSON(w, http.StatusOK, toInboundDTO(in))

	case http.MethodPut:
		if !s.requireCSRF(w, r, sess) {
			return
		}
		var p inboundPayload
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
			return
		}
		if err := validateInbound(p); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		err := db.UpdateInbound(s.db, db.Inbound{
			ID: id, Tag: p.Tag, Protocol: p.Protocol, Port: p.Port, Listen: p.Listen,
			SettingsJSON: p.SettingsJSON, StreamJSON: p.StreamJSON,
			SniffingJSON: p.SniffingJSON, Enabled: p.Enabled,
		})
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		_ = audit.Log(s.db, sess.AdminID, "inbound_update", p.Tag, clientIP(r))
		s.requestReload()
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})

	case http.MethodDelete:
		if !s.requireCSRF(w, r, sess) {
			return
		}
		if err := db.DeleteInbound(s.db, id); err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		_ = audit.Log(s.db, sess.AdminID, "inbound_delete", idStr, clientIP(r))
		s.requestReload()
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func validateInbound(p inboundPayload) error {
	if strings.TrimSpace(p.Tag) == "" {
		return errors.New("tag is required")
	}
	if strings.TrimSpace(p.Protocol) == "" {
		return errors.New("protocol is required")
	}
	if p.Port < 1 || p.Port > 65535 {
		return errors.New("port must be 1..65535")
	}
	if p.Listen == "" {
		p.Listen = "0.0.0.0"
	}
	for _, s := range []string{p.SettingsJSON, p.StreamJSON, p.SniffingJSON} {
		if err := db.ValidateJSON(s); err != nil {
			return err
		}
	}
	return nil
}
