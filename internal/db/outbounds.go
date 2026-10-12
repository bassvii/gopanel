// SPDX-License-Identifier: AGPL-3.0-or-later

package db

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Outbound — точка выхода трафика.
type Outbound struct {
	ID           int64
	Tag          string
	Protocol     string
	SettingsJSON string
	StreamJSON   string
	Enabled      bool
	IsBuiltin    bool
	CreatedAt    time.Time
}

// CreateOutbound создаёт outbound.
func CreateOutbound(conn *sql.DB, o Outbound) (int64, error) {
	if o.Tag == "" {
		return 0, errors.New("tag is required")
	}
	if o.Protocol == "" {
		return 0, errors.New("protocol is required")
	}
	if o.SettingsJSON == "" {
		o.SettingsJSON = "{}"
	}
	if o.StreamJSON == "" {
		o.StreamJSON = "{}"
	}

	res, err := conn.Exec(
		`INSERT INTO outbounds
		 (tag, protocol, settings_json, stream_json, enabled, is_builtin, created_at)
		 VALUES (?, ?, ?, ?, ?, 0, ?)`,
		o.Tag, o.Protocol, o.SettingsJSON, o.StreamJSON,
		boolToInt(o.Enabled), time.Now().Unix(),
	)
	if err != nil {
		return 0, fmt.Errorf("insert outbound: %w", err)
	}
	return res.LastInsertId()
}

// GetOutbound возвращает outbound по id.
func GetOutbound(conn *sql.DB, id int64) (Outbound, error) {
	var (
		o         Outbound
		enabled   int
		builtin   int
		createdAt int64
	)
	err := conn.QueryRow(
		`SELECT id, tag, protocol, settings_json, stream_json, enabled, is_builtin, created_at
		 FROM outbounds WHERE id = ?`,
		id,
	).Scan(&o.ID, &o.Tag, &o.Protocol, &o.SettingsJSON, &o.StreamJSON,
		&enabled, &builtin, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return o, ErrNotFound
	}
	if err != nil {
		return o, fmt.Errorf("get outbound: %w", err)
	}
	o.Enabled = enabled != 0
	o.IsBuiltin = builtin != 0
	o.CreatedAt = time.Unix(createdAt, 0)
	return o, nil
}

// GetOutboundByTag возвращает outbound по тегу.
func GetOutboundByTag(conn *sql.DB, tag string) (Outbound, error) {
	var id int64
	err := conn.QueryRow(`SELECT id FROM outbounds WHERE tag = ?`, tag).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return Outbound{}, ErrNotFound
	}
	if err != nil {
		return Outbound{}, err
	}
	return GetOutbound(conn, id)
}

// ListOutbounds возвращает все outbound'ы.
func ListOutbounds(conn *sql.DB) ([]Outbound, error) {
	rows, err := conn.Query(
		`SELECT id, tag, protocol, settings_json, stream_json, enabled, is_builtin, created_at
		 FROM outbounds ORDER BY is_builtin DESC, id`,
	)
	if err != nil {
		return nil, fmt.Errorf("list outbounds: %w", err)
	}
	defer rows.Close()

	var out []Outbound
	for rows.Next() {
		var (
			o         Outbound
			enabled   int
			builtin   int
			createdAt int64
		)
		if err := rows.Scan(&o.ID, &o.Tag, &o.Protocol, &o.SettingsJSON, &o.StreamJSON,
			&enabled, &builtin, &createdAt); err != nil {
			return nil, fmt.Errorf("scan outbound: %w", err)
		}
		o.Enabled = enabled != 0
		o.IsBuiltin = builtin != 0
		o.CreatedAt = time.Unix(createdAt, 0)
		out = append(out, o)
	}
	return out, rows.Err()
}

// UpdateOutbound обновляет outbound.
// Для builtin можно менять только enabled.
func UpdateOutbound(conn *sql.DB, o Outbound) error {
	if o.ID == 0 {
		return errors.New("id is required")
	}

	existing, err := GetOutbound(conn, o.ID)
	if err != nil {
		return err
	}

	if existing.IsBuiltin {
		// У builtin меняем только enabled.
		_, err := conn.Exec(
			`UPDATE outbounds SET enabled = ? WHERE id = ?`,
			boolToInt(o.Enabled), o.ID,
		)
		return err
	}

	_, err = conn.Exec(
		`UPDATE outbounds
		 SET tag = ?, protocol = ?, settings_json = ?, stream_json = ?, enabled = ?
		 WHERE id = ?`,
		o.Tag, o.Protocol, o.SettingsJSON, o.StreamJSON, boolToInt(o.Enabled), o.ID,
	)
	if err != nil {
		return fmt.Errorf("update outbound: %w", err)
	}
	return nil
}

// DeleteOutbound удаляет outbound. Builtin удалять нельзя.
func DeleteOutbound(conn *sql.DB, id int64) error {
	o, err := GetOutbound(conn, id)
	if err != nil {
		return err
	}
	if o.IsBuiltin {
		return errors.New("cannot delete builtin outbound")
	}
	_, err = conn.Exec(`DELETE FROM outbounds WHERE id = ?`, id)
	return err
}
