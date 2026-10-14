// SPDX-License-Identifier: AGPL-3.0-or-later

package db

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/bassvii/gopanel/internal/auth"
)

// migrateSecrets шифрует все TOTP-секреты (totp_secret и
// totp_pending_secret), которые ещё хранятся открытым текстом.
//
// Идемпотентна: если все секреты уже в формате enc:v1:, ничего
// не делает. Вызывается один раз при старте из Open, до того как
// панель начнёт обслуживать запросы.
//
// Ошибки не блокируют работу панели: если миграция упала (например,
// повреждён ключ), в лог панели попадёт сообщение — но панель
// продолжит запускаться, потому что старый открытый секрет всё
// ещё читается (см. decryptTOTPFields).
func migrateSecrets(conn *sql.DB) error {
	key := auth.SecretKey()
	if len(key) == 0 {
		return errors.New("secret key not loaded")
	}

	rows, err := conn.Query(
		`SELECT id, totp_secret, totp_pending_secret FROM admins`,
	)
	if err != nil {
		return fmt.Errorf("select admins: %w", err)
	}
	defer rows.Close()

	type fix struct {
		id              int64
		secret          string
		pending         string
		encSecret       string
		encPending      string
	}

	var fixes []fix

	for rows.Next() {
		var (
			id      int64
			secret  string
			pending string
		)
		if err := rows.Scan(&id, &secret, &pending); err != nil {
			return fmt.Errorf("scan admin: %w", err)
		}

		needsSecret := secret != "" && !auth.IsEncrypted(secret)
		needsPending := pending != "" && !auth.IsEncrypted(pending)

		if !needsSecret && !needsPending {
			continue
		}

		f := fix{id: id, secret: secret, pending: pending}

		if needsSecret {
			enc, err := auth.EncryptSecret(key, secret)
			if err != nil {
				return fmt.Errorf("encrypt totp_secret for admin %d: %w", id, err)
			}
			f.encSecret = enc
		} else {
			f.encSecret = secret
		}

		if needsPending {
			enc, err := auth.EncryptSecret(key, pending)
			if err != nil {
				return fmt.Errorf("encrypt totp_pending_secret for admin %d: %w", id, err)
			}
			f.encPending = enc
		} else {
			f.encPending = pending
		}

		fixes = append(fixes, f)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate admins: %w", err)
	}

	if len(fixes) == 0 {
		return nil
	}

	tx, err := conn.Begin()
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback()

	for _, f := range fixes {
		if _, err := tx.Exec(
			`UPDATE admins SET totp_secret = ?, totp_pending_secret = ? WHERE id = ?`,
			f.encSecret, f.encPending, f.id,
		); err != nil {
			return fmt.Errorf("update admin %d: %w", f.id, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	return nil
}
