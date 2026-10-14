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

type routingRuleDTO struct {
	ID              int64  `json:"id"`
	Position        int    `json:"position"`
	RuleTag         string `json:"rule_tag"`
	DomainsJSON     string `json:"domains_json"`
	IPsJSON         string `json:"ips_json"`
	PortsJSON       string `json:"ports_json"`
	SourceIPsJSON   string `json:"source_ips_json"`
	Network         string `json:"network"`
	InboundTagsJSON string `json:"inbound_tags_json"`
	ProtocolJSON    string `json:"protocol_json"`
	OutboundTag     string `json:"outbound_tag"`
	Enabled         bool   `json:"enabled"`
	CreatedAt       int64  `json:"created_at"`
}

type routingRulePayload struct {
	RuleTag         string `json:"rule_tag"`
	DomainsJSON     string `json:"domains_json"`
	IPsJSON         string `json:"ips_json"`
	PortsJSON       string `json:"ports_json"`
	SourceIPsJSON   string `json:"source_ips_json"`
	Network         string `json:"network"`
	InboundTagsJSON string `json:"inbound_tags_json"`
	ProtocolJSON    string `json:"protocol_json"`
	OutboundTag     string `json:"outbound_tag"`
	Enabled         bool   `json:"enabled"`
}

func toRoutingRuleDTO(r db.RoutingRule) routingRuleDTO {
	return routingRuleDTO{
		ID:              r.ID,
		Position:        r.Position,
		RuleTag:         r.RuleTag,
		DomainsJSON:     r.DomainsJSON,
		IPsJSON:         r.IPsJSON,
		PortsJSON:       r.PortsJSON,
		SourceIPsJSON:   r.SourceIPsJSON,
		Network:         r.Network,
		InboundTagsJSON: r.InboundTagsJSON,
		ProtocolJSON:    r.ProtocolJSON,
		OutboundTag:     r.OutboundTag,
		Enabled:         r.Enabled,
		CreatedAt:       r.CreatedAt.Unix(),
	}
}

// handleRoutingRules — GET /routing/rules, POST /routing/rules
func (s *Server) handleRoutingRules(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	switch r.Method {
	case http.MethodGet:
		list, err := db.ListRoutingRules(s.db)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		dtos := make([]routingRuleDTO, 0, len(list))
		for _, rule := range list {
			dtos = append(dtos, toRoutingRuleDTO(rule))
		}
		writeJSON(w, http.StatusOK, map[string]any{"rules": dtos})

	case http.MethodPost:
		if !s.requireCSRF(w, r, sess) {
			return
		}
		var p routingRulePayload
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
			return
		}
		if err := validateRoutingRule(p); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		id, err := db.CreateRoutingRule(s.db, db.RoutingRule{
			RuleTag:         p.RuleTag,
			DomainsJSON:     p.DomainsJSON,
			IPsJSON:         p.IPsJSON,
			PortsJSON:       p.PortsJSON,
			SourceIPsJSON:   p.SourceIPsJSON,
			Network:         p.Network,
			InboundTagsJSON: p.InboundTagsJSON,
			ProtocolJSON:    p.ProtocolJSON,
			OutboundTag:     p.OutboundTag,
			Enabled:         p.Enabled,
		})
		if err != nil {
			s.log.Error("create routing rule failed", "err", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
			return
		}
		_ = audit.Log(s.db, sess.AdminID, "routing_create", p.RuleTag, s.clientIP(r))
		s.requestReload()
		writeJSON(w, http.StatusOK, map[string]any{"id": id})

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleRoutingRule — GET/PUT/DELETE /routing/rules/{id}
func (s *Server) handleRoutingRule(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	// /routing/rules/{id}
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		rule, err := db.GetRoutingRule(s.db, id)
		if errors.Is(err, db.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, toRoutingRuleDTO(rule))

	case http.MethodPut:
		if !s.requireCSRF(w, r, sess) {
			return
		}
		var p routingRulePayload
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
			return
		}
		if err := validateRoutingRule(p); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		err := db.UpdateRoutingRule(s.db, db.RoutingRule{
			ID:              id,
			RuleTag:         p.RuleTag,
			DomainsJSON:     p.DomainsJSON,
			IPsJSON:         p.IPsJSON,
			PortsJSON:       p.PortsJSON,
			SourceIPsJSON:   p.SourceIPsJSON,
			Network:         p.Network,
			InboundTagsJSON: p.InboundTagsJSON,
			ProtocolJSON:    p.ProtocolJSON,
			OutboundTag:     p.OutboundTag,
			Enabled:         p.Enabled,
		})
		if err != nil {
			s.log.Error("update routing rule failed", "err", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
			return
		}
		_ = audit.Log(s.db, sess.AdminID, "routing_update", p.RuleTag, s.clientIP(r))
		s.requestReload()
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})

	case http.MethodDelete:
		if !s.requireCSRF(w, r, sess) {
			return
		}
		if err := db.DeleteRoutingRule(s.db, id); err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		_ = audit.Log(s.db, sess.AdminID, "routing_delete", idStr, s.clientIP(r))
		s.requestReload()
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleRoutingRuleMoveRoute — POST /routing/rules/{id}/move?direction=up|down
func (s *Server) handleRoutingRuleMoveRoute(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireSession(w, r)
	if !ok {
		return
	}
	if !s.requireCSRF(w, r, sess) {
		return
	}

	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	dir := r.URL.Query().Get("direction")
	if dir != "up" && dir != "down" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "direction must be up or down"})
		return
	}
	if err := db.MoveRoutingRule(s.db, id, dir); err != nil {
		s.log.Error("move routing rule failed", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	s.requestReload()
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func validateRoutingRule(p routingRulePayload) error {
	if strings.TrimSpace(p.OutboundTag) == "" {
		return errors.New("outbound_tag is required")
	}
	for _, s := range []string{p.DomainsJSON, p.IPsJSON, p.PortsJSON, p.SourceIPsJSON, p.InboundTagsJSON, p.ProtocolJSON} {
		if err := db.ValidateJSON(s); err != nil {
			return err
		}
	}
	return nil
}
