// SPDX-License-Identifier: AGPL-3.0-or-later

package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

// SessionTTL — время жизни сессии.
const SessionTTL = 30 * 24 * time.Hour

// ErrSessionNotFound возвращается, когда сессии нет или она истекла.
var ErrSessionNotFound = errors.New("session not found")

// Session — запись сессии администратора.
type Session struct {
	ID        string
	AdminID   int64
	IP        string
	UserAgent string
	LastSeen  time.Time
	ExpiresAt time.Time
	CSRFToken string
}

// NewSession создаёт сессию и возвращает её ID (токен для cookie).
// В базе хранится только хеш токена.
func NewSession(conn *sql.DB, adminID int64, ip, userAgent string) (string, Session, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", Session{}, fmt.Errorf("read token: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	id := tokenHash(token)

	csrfRaw := make([]byte, 32)
	if _, err := rand.Read(csrfRaw); err != nil {
		return "", Session{}, fmt.Errorf("read csrf: %w", err)
	}
	csrf := base64.RawURLEncoding.EncodeToString(csrfRaw)

	now := time.Now()
	expires := now.Add(SessionTTL)

	_, err := conn.Exec(
		`INSERT INTO sessions (id, admin_id, token_hash, ip, user_agent, last_seen, expires_at, csrf_token)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		id, adminID, id, ip, userAgent, now.Unix(), expires.Unix(), csrf,
	)
	if err != nil {
		return "", Session{}, fmt.Errorf("insert session: %w", err)
	}

	return token, Session{
		ID:        id,
		AdminID:   adminID,
		IP:        ip,
		UserAgent: userAgent,
		LastSeen:  now,
		ExpiresAt: expires,
		CSRFToken: csrf,
	}, nil
}

// GetSession ищет сессию по токену из cookie.
// Если сессия истекла — удаляет её и возвращает ErrSessionNotFound.
func GetSession(conn *sql.DB, token string) (Session, error) {
	if token == "" {
		return Session{}, ErrSessionNotFound
	}
	id := tokenHash(token)

	var (
		s         Session
		lastSeen  int64
		expiresAt int64
	)
	err := conn.QueryRow(
		`SELECT id, admin_id, ip, user_agent, last_seen, expires_at, csrf_token
		 FROM sessions WHERE id = ?`,
		id,
	).Scan(&s.ID, &s.AdminID, &s.IP, &s.UserAgent, &lastSeen, &expiresAt, &s.CSRFToken)
	if errors.Is(err, sql.ErrNoRows) {
		return Session{}, ErrSessionNotFound
	}
	if err != nil {
		return Session{}, fmt.Errorf("get session: %w", err)
	}

	s.LastSeen = time.Unix(lastSeen, 0)
	s.ExpiresAt = time.Unix(expiresAt, 0)

	if time.Now().After(s.ExpiresAt) {
		_ = DeleteSession(conn, s.ID)
		return Session{}, ErrSessionNotFound
	}
	return s, nil
}

// TouchSession обновляет last_seen.
func TouchSession(conn *sql.DB, id string) error {
	_, err := conn.Exec(
		`UPDATE sessions SET last_seen = ? WHERE id = ?`,
		time.Now().Unix(), id,
	)
	return err
}

// DeleteSession удаляет сессию по её ID (хешу токена).
func DeleteSession(conn *sql.DB, id string) error {
	_, err := conn.Exec(`DELETE FROM sessions WHERE id = ?`, id)
	return err
}

// DeleteAllSessionsForAdmin завершает все сессии администратора.
func DeleteAllSessionsForAdmin(conn *sql.DB, adminID int64) error {
	_, err := conn.Exec(`DELETE FROM sessions WHERE admin_id = ?`, adminID)
	return err
}

// CleanupExpiredSessions удаляет истёкшие сессии.
func CleanupExpiredSessions(conn *sql.DB) (int64, error) {
	res, err := conn.Exec(`DELETE FROM sessions WHERE expires_at < ?`, time.Now().Unix())
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// tokenHash возвращает sha256-хеш токена в hex.
// Токен длинный и случайный, поэтому sha256 достаточно (argon2 тут избыточен).
func tokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// ListSessions возвращает все сессии администратора.
func ListSessions(conn *sql.DB, adminID int64) ([]Session, error) {
	rows, err := conn.Query(
		`SELECT id, admin_id, ip, user_agent, last_seen, expires_at, csrf_token
		 FROM sessions WHERE admin_id = ? ORDER BY last_seen DESC`,
		adminID,
	)
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}
	defer rows.Close()

	var out []Session
	for rows.Next() {
		var (
			s         Session
			lastSeen  int64
			expiresAt int64
		)
		if err := rows.Scan(&s.ID, &s.AdminID, &s.IP, &s.UserAgent,
			&lastSeen, &expiresAt, &s.CSRFToken); err != nil {
			return nil, err
		}
		s.LastSeen = time.Unix(lastSeen, 0)
		s.ExpiresAt = time.Unix(expiresAt, 0)
		out = append(out, s)
	}
	return out, rows.Err()
}

// DeleteOtherSessions удаляет все сессии администратора, кроме указанной.
func DeleteOtherSessions(conn *sql.DB, adminID int64, keepSessionID string) error {
	_, err := conn.Exec(
		`DELETE FROM sessions WHERE admin_id = ? AND id != ?`,
		adminID, keepSessionID,
	)
	return err
}
