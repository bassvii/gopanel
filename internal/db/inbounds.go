// SPDX-License-Identifier: AGPL-3.0-or-later

package db

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// ErrNotFound возвращается, когда запись не найдена.
var ErrNotFound = errors.New("not found")

// Inbound — инбаунд ядра.
type Inbound struct {
	ID           int64
	Tag          string
	Protocol     string
	Port         int
	Listen       string
	SettingsJSON string
	StreamJSON   string
	SniffingJSON string
	Enabled      bool
	CreatedAt    time.Time
}

// CreateInbound создаёт инбаунд.
func CreateInbound(conn *sql.DB, in Inbound) (int64, error) {
	if in.Tag == "" {
		return 0, errors.New("tag is required")
	}
	if in.Protocol == "" {
		return 0, errors.New("protocol is required")
	}
	if in.Port < 1 || in.Port > 65535 {
		return 0, fmt.Errorf("invalid port: %d", in.Port)
	}
	if in.Listen == "" {
		in.Listen = "0.0.0.0"
	}
	if in.SettingsJSON == "" {
		in.SettingsJSON = "{}"
	}
	if in.StreamJSON == "" {
		in.StreamJSON = "{}"
	}
	if in.SniffingJSON == "" {
		in.SniffingJSON = "{}"
	}

	res, err := conn.Exec(
		`INSERT INTO inbounds
		 (tag, protocol, port, listen, settings_json, stream_json, sniffing_json, enabled, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		in.Tag, in.Protocol, in.Port, in.Listen,
		in.SettingsJSON, in.StreamJSON, in.SniffingJSON,
		boolToInt(in.Enabled), time.Now().Unix(),
	)
	if err != nil {
		return 0, fmt.Errorf("insert inbound: %w", err)
	}
	return res.LastInsertId()
}

// GetInbound возвращает инбаунд по id.
func GetInbound(conn *sql.DB, id int64) (Inbound, error) {
	var (
		in        Inbound
		enabled   int
		createdAt int64
	)
	err := conn.QueryRow(
		`SELECT id, tag, protocol, port, listen, settings_json, stream_json,
		        sniffing_json, enabled, created_at
		 FROM inbounds WHERE id = ?`,
		id,
	).Scan(&in.ID, &in.Tag, &in.Protocol, &in.Port, &in.Listen,
		&in.SettingsJSON, &in.StreamJSON, &in.SniffingJSON,
		&enabled, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return in, ErrNotFound
	}
	if err != nil {
		return in, fmt.Errorf("get inbound: %w", err)
	}
	in.Enabled = enabled != 0
	in.CreatedAt = time.Unix(createdAt, 0)
	return in, nil
}

// GetInboundByTag возвращает инбаунд по тегу.
func GetInboundByTag(conn *sql.DB, tag string) (Inbound, error) {
	var id int64
	err := conn.QueryRow(`SELECT id FROM inbounds WHERE tag = ?`, tag).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return Inbound{}, ErrNotFound
	}
	if err != nil {
		return Inbound{}, err
	}
	return GetInbound(conn, id)
}

// ListInbounds возвращает все инбаунды (в порядке создания).
func ListInbounds(conn *sql.DB) ([]Inbound, error) {
	rows, err := conn.Query(
		`SELECT id, tag, protocol, port, listen, settings_json, stream_json,
		        sniffing_json, enabled, created_at
		 FROM inbounds ORDER BY id`,
	)
	if err != nil {
		return nil, fmt.Errorf("list inbounds: %w", err)
	}
	defer rows.Close()

	var out []Inbound
	for rows.Next() {
		var (
			in        Inbound
			enabled   int
			createdAt int64
		)
		if err := rows.Scan(&in.ID, &in.Tag, &in.Protocol, &in.Port, &in.Listen,
			&in.SettingsJSON, &in.StreamJSON, &in.SniffingJSON,
			&enabled, &createdAt); err != nil {
			return nil, fmt.Errorf("scan inbound: %w", err)
		}
		in.Enabled = enabled != 0
		in.CreatedAt = time.Unix(createdAt, 0)
		out = append(out, in)
	}
	return out, rows.Err()
}

// UpdateInbound обновляет инбаунд.
func UpdateInbound(conn *sql.DB, in Inbound) error {
	if in.ID == 0 {
		return errors.New("id is required")
	}
	if in.Port < 1 || in.Port > 65535 {
		return fmt.Errorf("invalid port: %d", in.Port)
	}
	_, err := conn.Exec(
		`UPDATE inbounds
		 SET tag = ?, protocol = ?, port = ?, listen = ?,
		     settings_json = ?, stream_json = ?, sniffing_json = ?,
		     enabled = ?
		 WHERE id = ?`,
		in.Tag, in.Protocol, in.Port, in.Listen,
		in.SettingsJSON, in.StreamJSON, in.SniffingJSON,
		boolToInt(in.Enabled), in.ID,
	)
	if err != nil {
		return fmt.Errorf("update inbound: %w", err)
	}
	return nil
}

// DeleteInbound удаляет инбаунд (связи в user_inbound удалятся каскадом).
func DeleteInbound(conn *sql.DB, id int64) error {
	_, err := conn.Exec(`DELETE FROM inbounds WHERE id = ?`, id)
	return err
}

// ValidateJSON проверяет, что строка — валидный JSON-объект.
func ValidateJSON(s string) error {
	if s == "" {
		return nil
	}
	var v any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		return fmt.Errorf("invalid json: %w", err)
	}
	return nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
