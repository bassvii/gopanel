// SPDX-License-Identifier: AGPL-3.0-or-later

package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/bassvii/gopanel/internal/audit"
	"github.com/bassvii/gopanel/internal/db"
)

// presetRule — описание одного правила в пресете.
type presetRule struct {
	RuleTag     string
	Domains     []string
	IPs         []string
	Network     string
	OutboundTag string
}

// preset — именованный набор правил.
type preset struct {
	Name        string
	Description string
	Rules       []presetRule
}

var presets = map[string]preset{
	"ru-direct": {
		Name:        "Российские сайты — напрямую",
		Description: "РФ домены и IP идут напрямую, минуя proxy",
		Rules: []presetRule{
			{
				RuleTag:     "РФ и приватные — DIRECT",
				Domains:     []string{"geosite:category-ru", "geosite:private"},
				IPs:         []string{"geoip:ru", "geoip:private"},
				OutboundTag: "DIRECT",
			},
		},
	},
	"ads-block": {
		Name:        "Реклама — блокировать",
		Description: "Рекламные домены отбрасываются",
		Rules: []presetRule{
			{
				RuleTag:     "Реклама — BLOCK",
				Domains:     []string{"geosite:category-ads-all"},
				OutboundTag: "BLOCK",
			},
		},
	},
	"private-direct": {
		Name: "Приватные IP — напрямую (не рекомендуется)",
		Description: "Попытка направить локальные адреса в DIRECT. " +
			"НЕ СРАБОТАЕТ: встроенное правило безопасности блокирует " +
			"geoip:private раньше. Оставлено для совместимости.",
		Rules: []presetRule{
			{
				RuleTag:     "Приватные — DIRECT",
				Domains:     []string{"geosite:private"},
				IPs:         []string{"geoip:private"},
				OutboundTag: "DIRECT",
			},
		},
	},
	"everything-proxy": {
		Name:        "Всё остальное — через proxy",
		Description: "Правило по умолчанию: весь остальной трафик идёт через PROXY",
		Rules: []presetRule{
			{
				RuleTag:     "Всё остальное — PROXY",
				Network:     "tcp,udp",
				OutboundTag: "PROXY",
			},
		},
	},
	"full-ru-split": {
		Name:        "Полный РФ-сплит",
		Description: "Реклама — блок, РФ — напрямую, всё остальное — через PROXY",
		Rules: []presetRule{
			{
				RuleTag:     "Реклама — BLOCK",
				Domains:     []string{"geosite:category-ads-all"},
				OutboundTag: "BLOCK",
			},
			{
				RuleTag:     "РФ и приватные — DIRECT",
				Domains:     []string{"geosite:category-ru", "geosite:private"},
				IPs:         []string{"geoip:ru", "geoip:private"},
				OutboundTag: "DIRECT",
			},
			{
				RuleTag:     "Всё остальное — PROXY",
				Network:     "tcp,udp",
				OutboundTag: "PROXY",
			},
		},
	},
}

type presetInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	RuleCount   int    `json:"rule_count"`
}

// handleRoutingPresets — GET /routing/presets
func (s *Server) handleRoutingPresets(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireSession(w, r); !ok {
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	out := make([]presetInfo, 0, len(presets))
	// Порядок вывода — стабильный.
	order := []string{"full-ru-split", "ru-direct", "ads-block", "private-direct", "everything-proxy"}
	for _, name := range order {
		p, ok := presets[name]
		if !ok {
			continue
		}
		out = append(out, presetInfo{
			Name:        name,
			Description: p.Description,
			RuleCount:   len(p.Rules),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"presets": out})
}

// handleRoutingPresetApply — POST /routing/presets/{name}
func (s *Server) handleRoutingPresetApply(w http.ResponseWriter, r *http.Request) {
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

	name := r.PathValue("name")
	p, ok := presets[name]
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "unknown preset"})
		return
	}

	// Проверяем, что нужные outbound'ы существуют.
	// DIRECT/BLOCK — builtin. PROXY должен быть создан пользователем.
	for _, rule := range p.Rules {
		if _, err := db.GetOutboundByTag(s.db, rule.OutboundTag); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": fmt.Sprintf("outbound %q не найден — создайте его перед применением пресета", rule.OutboundTag),
			})
			return
		}
	}

	created, updated, err := s.applyPresetRules(p)
	if err != nil {
		s.log.Error("apply preset failed", "err", err, "preset", name)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	_ = audit.Log(s.db, sess.AdminID, "routing_preset_apply", name, s.clientIP(r))
	resp := map[string]any{
		"ok":      true,
		"created": created,
		"updated": updated,
		"preset":  name,
		"applied": true,
	}
	if err := s.applyNow(); err != nil {
		resp["applied"] = false
		resp["apply_error"] = err.Error()
	}
	writeJSON(w, http.StatusOK, resp)
}

// applyPresetRules применяет правила пресета идемпотентно:
// если правило с таким RuleTag уже есть — обновляет его,
// иначе создаёт. Возвращает (created, updated, err).
//
// Всё в одной транзакции: если упадёт хоть одно правило,
// БД останется в исходном состоянии. См. #4 нового аудита.
func (s *Server) applyPresetRules(p preset) (int, int, error) {
	existing, err := db.ListRoutingRules(s.db)
	if err != nil {
		return 0, 0, err
	}
	byTag := make(map[string]db.RoutingRule, len(existing))
	for _, r := range existing {
		if r.RuleTag != "" {
			byTag[r.RuleTag] = r
		}
	}

	tx, err := s.db.Begin()
	if err != nil {
		return 0, 0, err
	}
	defer tx.Rollback()

	created := 0
	updated := 0
	for _, rule := range p.Rules {
		domainsJSON, _ := json.Marshal(rule.Domains)
		ipsJSON, _ := json.Marshal(rule.IPs)

		row := db.RoutingRule{
			RuleTag:         rule.RuleTag,
			DomainsJSON:     string(domainsJSON),
			IPsJSON:         string(ipsJSON),
			PortsJSON:       "[]",
			SourceIPsJSON:   "[]",
			Network:         rule.Network,
			InboundTagsJSON: "[]",
			ProtocolJSON:    "[]",
			OutboundTag:     rule.OutboundTag,
			Enabled:         true,
		}

		if prev, ok := byTag[rule.RuleTag]; ok {
			row.ID = prev.ID
			if err := db.UpdateRoutingRuleTx(tx, row); err != nil {
				return 0, 0, err
			}
			updated++
		} else {
			if err := db.CreateRoutingRuleTx(tx, row); err != nil {
				return 0, 0, err
			}
			created++
		}
	}

	return created, updated, tx.Commit()
}
