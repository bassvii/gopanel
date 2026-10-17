// SPDX-License-Identifier: AGPL-3.0-or-later

package db

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// RoutingRule — правило маршрутизации.
type RoutingRule struct {
	ID              int64
	Position        int
	RuleTag         string
	DomainsJSON     string
	IPsJSON         string
	PortsJSON       string
	SourceIPsJSON   string
	Network         string
	InboundTagsJSON string
	ProtocolJSON    string
	OutboundTag     string
	Enabled         bool
	CreatedAt       time.Time
}

// CreateRoutingRule создаёт правило.
// Позиция автоматически = max + 1.
func CreateRoutingRule(conn *sql.DB, r RoutingRule) (int64, error) {
	if r.OutboundTag == "" {
		return 0, errors.New("outbound_tag is required")
	}
	r.DomainsJSON = defaultJSONArray(r.DomainsJSON)
	r.IPsJSON = defaultJSONArray(r.IPsJSON)
	r.PortsJSON = defaultJSONArray(r.PortsJSON)
	r.SourceIPsJSON = defaultJSONArray(r.SourceIPsJSON)
	r.InboundTagsJSON = defaultJSONArray(r.InboundTagsJSON)
	r.ProtocolJSON = defaultJSONArray(r.ProtocolJSON)

	tx, err := conn.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	// Следующая позиция.
	var maxPos sql.NullInt64
	if err := tx.QueryRow(`SELECT MAX(position) FROM routing_rules`).Scan(&maxPos); err != nil {
		return 0, err
	}
	pos := 1
	if maxPos.Valid {
		pos = int(maxPos.Int64) + 1
	}

	res, err := tx.Exec(
		`INSERT INTO routing_rules
		 (position, rule_tag, domains_json, ips_json, ports_json, source_ips_json,
		  network, inbound_tags_json, protocol_json, outbound_tag, enabled, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		pos, r.RuleTag, r.DomainsJSON, r.IPsJSON, r.PortsJSON, r.SourceIPsJSON,
		r.Network, r.InboundTagsJSON, r.ProtocolJSON, r.OutboundTag,
		boolToInt(r.Enabled), time.Now().Unix(),
	)
	if err != nil {
		return 0, fmt.Errorf("insert routing rule: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	return id, tx.Commit()
}

// GetRoutingRule возвращает правило по id.
func GetRoutingRule(conn *sql.DB, id int64) (RoutingRule, error) {
	var (
		r         RoutingRule
		enabled   int
		createdAt int64
	)
	err := conn.QueryRow(
		`SELECT id, position, rule_tag, domains_json, ips_json, ports_json,
		        source_ips_json, network, inbound_tags_json, protocol_json,
		        outbound_tag, enabled, created_at
		 FROM routing_rules WHERE id = ?`,
		id,
	).Scan(&r.ID, &r.Position, &r.RuleTag, &r.DomainsJSON, &r.IPsJSON, &r.PortsJSON,
		&r.SourceIPsJSON, &r.Network, &r.InboundTagsJSON, &r.ProtocolJSON,
		&r.OutboundTag, &enabled, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return r, ErrNotFound
	}
	if err != nil {
		return r, fmt.Errorf("get routing rule: %w", err)
	}
	r.Enabled = enabled != 0
	r.CreatedAt = time.Unix(createdAt, 0)
	return r, nil
}

// ListRoutingRules возвращает все правила по порядку.
func ListRoutingRules(conn *sql.DB) ([]RoutingRule, error) {
	rows, err := conn.Query(
		`SELECT id, position, rule_tag, domains_json, ips_json, ports_json,
		        source_ips_json, network, inbound_tags_json, protocol_json,
		        outbound_tag, enabled, created_at
		 FROM routing_rules ORDER BY position`,
	)
	if err != nil {
		return nil, fmt.Errorf("list routing rules: %w", err)
	}
	defer rows.Close()

	var out []RoutingRule
	for rows.Next() {
		var (
			r         RoutingRule
			enabled   int
			createdAt int64
		)
		if err := rows.Scan(&r.ID, &r.Position, &r.RuleTag, &r.DomainsJSON, &r.IPsJSON,
			&r.PortsJSON, &r.SourceIPsJSON, &r.Network, &r.InboundTagsJSON,
			&r.ProtocolJSON, &r.OutboundTag, &enabled, &createdAt); err != nil {
			return nil, fmt.Errorf("scan routing rule: %w", err)
		}
		r.Enabled = enabled != 0
		r.CreatedAt = time.Unix(createdAt, 0)
		out = append(out, r)
	}
	return out, rows.Err()
}

// UpdateRoutingRule обновляет правило (кроме position).
func UpdateRoutingRule(conn *sql.DB, r RoutingRule) error {
	if r.ID == 0 {
		return errors.New("id is required")
	}
	if r.OutboundTag == "" {
		return errors.New("outbound_tag is required")
	}

	r.DomainsJSON = defaultJSONArray(r.DomainsJSON)
	r.IPsJSON = defaultJSONArray(r.IPsJSON)
	r.PortsJSON = defaultJSONArray(r.PortsJSON)
	r.SourceIPsJSON = defaultJSONArray(r.SourceIPsJSON)
	r.InboundTagsJSON = defaultJSONArray(r.InboundTagsJSON)
	r.ProtocolJSON = defaultJSONArray(r.ProtocolJSON)

	_, err := conn.Exec(
		`UPDATE routing_rules
		 SET rule_tag = ?, domains_json = ?, ips_json = ?, ports_json = ?,
		     source_ips_json = ?, network = ?, inbound_tags_json = ?,
		     protocol_json = ?, outbound_tag = ?, enabled = ?
		 WHERE id = ?`,
		r.RuleTag, r.DomainsJSON, r.IPsJSON, r.PortsJSON, r.SourceIPsJSON,
		r.Network, r.InboundTagsJSON, r.ProtocolJSON, r.OutboundTag,
		boolToInt(r.Enabled), r.ID,
	)
	if err != nil {
		return fmt.Errorf("update routing rule: %w", err)
	}
	return nil
}

// DeleteRoutingRule удаляет правило и нормализует позиции.
func DeleteRoutingRule(conn *sql.DB, id int64) error {
	tx, err := conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM routing_rules WHERE id = ?`, id); err != nil {
		return err
	}
	if err := normalizePositions(tx); err != nil {
		return err
	}
	return tx.Commit()
}

// MoveRoutingRule перемещает правило вверх или вниз.
// direction: "up" или "down".
func MoveRoutingRule(conn *sql.DB, id int64, direction string) error {
	tx, err := conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var pos int
	err = tx.QueryRow(`SELECT position FROM routing_rules WHERE id = ?`, id).Scan(&pos)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}

	var neighborID int64
	var neighborPos int
	switch direction {
	case "up":
		err = tx.QueryRow(
			`SELECT id, position FROM routing_rules WHERE position < ? ORDER BY position DESC LIMIT 1`,
			pos,
		).Scan(&neighborID, &neighborPos)
	case "down":
		err = tx.QueryRow(
			`SELECT id, position FROM routing_rules WHERE position > ? ORDER BY position ASC LIMIT 1`,
			pos,
		).Scan(&neighborID, &neighborPos)
	default:
		return fmt.Errorf("invalid direction: %s", direction)
	}
	if errors.Is(err, sql.ErrNoRows) {
		return nil // уже на краю — ничего не делаем
	}
	if err != nil {
		return err
	}

	// Меняем позиции местами.
	if _, err := tx.Exec(`UPDATE routing_rules SET position = ? WHERE id = ?`, neighborPos, id); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE routing_rules SET position = ? WHERE id = ?`, pos, neighborID); err != nil {
		return err
	}
	return tx.Commit()
}

// normalizePositions пересчитывает position подряд, начиная с 1.
func normalizePositions(tx *sql.Tx) error {
	rows, err := tx.Query(`SELECT id FROM routing_rules ORDER BY position`)
	if err != nil {
		return err
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	rows.Close()

	for i, id := range ids {
		if _, err := tx.Exec(`UPDATE routing_rules SET position = ? WHERE id = ?`, i+1, id); err != nil {
			return err
		}
	}
	return nil
}

func defaultJSONArray(s string) string {
	if s == "" {
		return "[]"
	}
	return s
}

// CreateRoutingRuleTx — вариант CreateRoutingRule для работы внутри
// уже открытой транзакции. Позиция = max + 1 в этой же транзакции.
func CreateRoutingRuleTx(tx *sql.Tx, r RoutingRule) error {
	if r.OutboundTag == "" {
		return errors.New("outbound_tag is required")
	}
	r.DomainsJSON = defaultJSONArray(r.DomainsJSON)
	r.IPsJSON = defaultJSONArray(r.IPsJSON)
	r.PortsJSON = defaultJSONArray(r.PortsJSON)
	r.SourceIPsJSON = defaultJSONArray(r.SourceIPsJSON)
	r.InboundTagsJSON = defaultJSONArray(r.InboundTagsJSON)
	r.ProtocolJSON = defaultJSONArray(r.ProtocolJSON)

	var maxPos sql.NullInt64
	if err := tx.QueryRow(`SELECT MAX(position) FROM routing_rules`).Scan(&maxPos); err != nil {
		return err
	}
	pos := 1
	if maxPos.Valid {
		pos = int(maxPos.Int64) + 1
	}

	_, err := tx.Exec(
		`INSERT INTO routing_rules
		 (position, rule_tag, domains_json, ips_json, ports_json, source_ips_json,
		  network, inbound_tags_json, protocol_json, outbound_tag, enabled, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		pos, r.RuleTag, r.DomainsJSON, r.IPsJSON, r.PortsJSON, r.SourceIPsJSON,
		r.Network, r.InboundTagsJSON, r.ProtocolJSON, r.OutboundTag,
		boolToInt(r.Enabled), time.Now().Unix(),
	)
	if err != nil {
		return fmt.Errorf("insert routing rule: %w", err)
	}
	return nil
}

// UpdateRoutingRuleTx — вариант UpdateRoutingRule для транзакции.
func UpdateRoutingRuleTx(tx *sql.Tx, r RoutingRule) error {
	if r.ID == 0 {
		return errors.New("id is required")
	}
	if r.OutboundTag == "" {
		return errors.New("outbound_tag is required")
	}

	r.DomainsJSON = defaultJSONArray(r.DomainsJSON)
	r.IPsJSON = defaultJSONArray(r.IPsJSON)
	r.PortsJSON = defaultJSONArray(r.PortsJSON)
	r.SourceIPsJSON = defaultJSONArray(r.SourceIPsJSON)
	r.InboundTagsJSON = defaultJSONArray(r.InboundTagsJSON)
	r.ProtocolJSON = defaultJSONArray(r.ProtocolJSON)

	_, err := tx.Exec(
		`UPDATE routing_rules
		 SET rule_tag = ?, domains_json = ?, ips_json = ?, ports_json = ?,
		     source_ips_json = ?, network = ?, inbound_tags_json = ?,
		     protocol_json = ?, outbound_tag = ?, enabled = ?
		 WHERE id = ?`,
		r.RuleTag, r.DomainsJSON, r.IPsJSON, r.PortsJSON, r.SourceIPsJSON,
		r.Network, r.InboundTagsJSON, r.ProtocolJSON, r.OutboundTag,
		boolToInt(r.Enabled), r.ID,
	)
	if err != nil {
		return fmt.Errorf("update routing rule: %w", err)
	}
	return nil
}

// CountRoutingRulesByOutbound возвращает число включённых правил,
// ссылающихся на указанный outbound_tag.
func CountRoutingRulesByOutbound(conn *sql.DB, tag string) (int, error) {
	var n int
	err := conn.QueryRow(
		`SELECT COUNT(*) FROM routing_rules WHERE outbound_tag = ? AND enabled = 1`,
		tag,
	).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count routing rules by outbound %q: %w", tag, err)
	}
	return n, nil
}
