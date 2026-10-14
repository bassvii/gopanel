// SPDX-License-Identifier: AGPL-3.0-or-later

package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"fmt"

	"github.com/bassvii/gopanel/internal/audit"
	"github.com/bassvii/gopanel/internal/db"
)

const settingAdminPort = "admin_port"

// adminPortFromDB читает порт админки из settings (0, если не задан).
// Нужен, чтобы запретить инбаундам занимать порт панели.
func adminPortFromDB(conn *sql.DB) int {
	v, ok, err := db.GetSetting(conn, settingAdminPort)
	if err != nil || !ok {
		return 0
	}
	p, err := strconv.Atoi(v)
	if err != nil {
		return 0
	}
	return p
}

type inboundDTO struct {
	ID           int64  `json:"id"`
	Tag          string `json:"tag"`
	Protocol     string `json:"protocol"`
	Port         int    `json:"port"`
	Listen       string `json:"listen"`
	SettingsJSON string `json:"settings_json"`
	StreamJSON   string `json:"stream_json"`
	SniffingJSON string `json:"sniffing_json"`
	RealityPublicKey string `json:"reality_public_key"`
	VlessEncClient   string `json:"vless_enc_client"`
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
		RealityPublicKey: in.RealityPublicKey,
		VlessEncClient:   in.VlessEncClient,
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
	RealityPublicKey string `json:"reality_public_key"`
	VlessEncClient   string `json:"vless_enc_client"`
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
		existing, err := db.ListInbounds(s.db)
		if err != nil {
			s.log.Error("list inbounds for validation failed", "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if err := validateInbound(p, adminPortFromDB(s.db), existing, 0); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}

		id, err := db.CreateInbound(s.db, db.Inbound{
			Tag: p.Tag, Protocol: p.Protocol, Port: p.Port, Listen: p.Listen,
			SettingsJSON: p.SettingsJSON, StreamJSON: p.StreamJSON,
			SniffingJSON: p.SniffingJSON, RealityPublicKey: p.RealityPublicKey,
			Enabled: p.Enabled,
		})
		if err != nil {
			s.log.Error("create inbound failed", "err", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		_ = audit.Log(s.db, sess.AdminID, "inbound_create", p.Tag, s.clientIP(r))
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

	idStr := r.PathValue("id")
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
		existing, err := db.ListInbounds(s.db)
		if err != nil {
			s.log.Error("list inbounds for validation failed", "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if err := validateInbound(p, adminPortFromDB(s.db), existing, id); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		err = db.UpdateInbound(s.db, db.Inbound{
			ID: id, Tag: p.Tag, Protocol: p.Protocol, Port: p.Port, Listen: p.Listen,
			SettingsJSON: p.SettingsJSON, StreamJSON: p.StreamJSON,
			SniffingJSON: p.SniffingJSON, RealityPublicKey: p.RealityPublicKey,
			Enabled: p.Enabled,
		})
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		_ = audit.Log(s.db, sess.AdminID, "inbound_update", p.Tag, s.clientIP(r))
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
		_ = audit.Log(s.db, sess.AdminID, "inbound_delete", idStr, s.clientIP(r))
		s.requestReload()
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// reservedPorts — порты, которые нельзя занимать инбаундом.
// 10085 — встроенный API Xray для StatsService.
// Порт админки панели проверяется отдельно (передаётся в validateInbound).
func validateInbound(p inboundPayload, adminPort int, existing []db.Inbound, excludeID int64) error {
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

	// Запрет на порт админки и StatsService.
	if p.Port == adminPort {
		return fmt.Errorf("port %d занят админкой панели", p.Port)
	}
	if p.Port == 10085 {
		return errors.New("порт 10085 зарезервирован для StatsService Xray")
	}

	// Уникальность (listen, port) среди включённых инбаундов.
	// Игнорируем excludeID — это случай, когда мы обновляем тот же инбаунд.
	for _, in := range existing {
		if in.ID == excludeID {
			continue
		}
		if !in.Enabled {
			continue
		}
		if in.Port == p.Port && in.Listen == p.Listen {
			return fmt.Errorf("порт %d на %s уже занят инбаундом %q", p.Port, p.Listen, in.Tag)
		}
	}

	for _, s := range []string{p.SettingsJSON, p.StreamJSON, p.SniffingJSON} {
		if err := db.ValidateJSON(s); err != nil {
			return err
		}
	}
	return nil
}
