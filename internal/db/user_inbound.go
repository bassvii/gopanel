// SPDX-License-Identifier: AGPL-3.0-or-later

package db

import (
	"database/sql"
	"errors"
	"fmt"
)

// UserInbound — связь пользователя с инбаундом.
type UserInbound struct {
	UserID         int64
	InboundID      int64
	CredentialJSON string
}

// AttachUserToInbound создаёт или обновляет связь.
func AttachUserToInbound(conn *sql.DB, ui UserInbound) error {
	if ui.CredentialJSON == "" {
		ui.CredentialJSON = "{}"
	}
	_, err := conn.Exec(
		`INSERT INTO user_inbound (user_id, inbound_id, credential_json)
		 VALUES (?, ?, ?)
		 ON CONFLICT(user_id, inbound_id) DO UPDATE SET credential_json = excluded.credential_json`,
		ui.UserID, ui.InboundID, ui.CredentialJSON,
	)
	if err != nil {
		return fmt.Errorf("attach user to inbound: %w", err)
	}
	return nil
}

// DetachUserFromInbound удаляет связь.
func DetachUserFromInbound(conn *sql.DB, userID, inboundID int64) error {
	_, err := conn.Exec(
		`DELETE FROM user_inbound WHERE user_id = ? AND inbound_id = ?`,
		userID, inboundID,
	)
	return err
}

// GetUserInbound возвращает связь.
func GetUserInbound(conn *sql.DB, userID, inboundID int64) (UserInbound, error) {
	var ui UserInbound
	err := conn.QueryRow(
		`SELECT user_id, inbound_id, credential_json
		 FROM user_inbound WHERE user_id = ? AND inbound_id = ?`,
		userID, inboundID,
	).Scan(&ui.UserID, &ui.InboundID, &ui.CredentialJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return ui, ErrNotFound
	}
	if err != nil {
		return ui, fmt.Errorf("get user_inbound: %w", err)
	}
	return ui, nil
}

// ListInboundsForUser возвращает все связи пользователя.
func ListInboundsForUser(conn *sql.DB, userID int64) ([]UserInbound, error) {
	rows, err := conn.Query(
		`SELECT user_id, inbound_id, credential_json
		 FROM user_inbound WHERE user_id = ? ORDER BY inbound_id`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("list user_inbound: %w", err)
	}
	defer rows.Close()

	var out []UserInbound
	for rows.Next() {
		var ui UserInbound
		if err := rows.Scan(&ui.UserID, &ui.InboundID, &ui.CredentialJSON); err != nil {
			return nil, fmt.Errorf("scan user_inbound: %w", err)
		}
		out = append(out, ui)
	}
	return out, rows.Err()
}

// ListUsersForInbound возвращает все связи инбаунда.
func ListUsersForInbound(conn *sql.DB, inboundID int64) ([]UserInbound, error) {
	rows, err := conn.Query(
		`SELECT user_id, inbound_id, credential_json
		 FROM user_inbound WHERE inbound_id = ? ORDER BY user_id`,
		inboundID,
	)
	if err != nil {
		return nil, fmt.Errorf("list user_inbound: %w", err)
	}
	defer rows.Close()

	var out []UserInbound
	for rows.Next() {
		var ui UserInbound
		if err := rows.Scan(&ui.UserID, &ui.InboundID, &ui.CredentialJSON); err != nil {
			return nil, fmt.Errorf("scan user_inbound: %w", err)
		}
		out = append(out, ui)
	}
	return out, rows.Err()
}
