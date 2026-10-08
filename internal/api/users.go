// SPDX-License-Identifier: AGPL-3.0-or-later

package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/bassvii/gopanel/internal/audit"
	"github.com/bassvii/gopanel/internal/db"
)

type userPayload struct {
	Name        string `json:"name"`
	Email       string `json:"email"`
	Enabled     bool   `json:"enabled"`
	QuotaBytes  int64  `json:"quota_bytes"`
	ExpiresAt   int64  `json:"expires_at"` // unix, 0 = бессрочно
	DeviceLimit int    `json:"device_limit"`
	Note        string `json:"note"`
}

func (s *Server) handleUsers(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	switch r.Method {
	case http.MethodGet:
		list, err := db.ListUsers(s.db)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"users": list})

	case http.MethodPost:
		if !s.requireCSRF(w, r, sess) {
			return
		}
		var p userPayload
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
			return
		}
		if strings.TrimSpace(p.Name) == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
			return
		}

		var expires time.Time
		if p.ExpiresAt > 0 {
			expires = time.Unix(p.ExpiresAt, 0)
		}

		id, err := db.CreateUser(s.db, db.User{
			Name:        p.Name,
			Email:       p.Email,
			Enabled:     p.Enabled,
			QuotaBytes:  p.QuotaBytes,
			ExpiresAt:   expires,
			DeviceLimit: p.DeviceLimit,
			Note:        p.Note,
		})
		if err != nil {
			s.log.Error("create user failed", "err", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		_ = audit.Log(s.db, sess.AdminID, "user_create", p.Name, clientIP(r))
		s.requestReload()
		writeJSON(w, http.StatusOK, map[string]any{"id": id})

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleUser(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	idStr := strings.TrimPrefix(r.URL.Path, s.secret+"/users/")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		u, err := db.GetUser(s.db, id)
		if errors.Is(err, db.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, u)

	case http.MethodPut:
		if !s.requireCSRF(w, r, sess) {
			return
		}
		var p userPayload
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
			return
		}
		var expires time.Time
		if p.ExpiresAt > 0 {
			expires = time.Unix(p.ExpiresAt, 0)
		}
		err := db.UpdateUser(s.db, db.User{
			ID: id, Name: p.Name, Email: p.Email, Enabled: p.Enabled,
			QuotaBytes: p.QuotaBytes, ExpiresAt: expires,
			DeviceLimit: p.DeviceLimit, Note: p.Note,
		})
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		_ = audit.Log(s.db, sess.AdminID, "user_update", p.Name, clientIP(r))
		s.requestReload()
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})

	case http.MethodDelete:
		if !s.requireCSRF(w, r, sess) {
			return
		}
		if err := db.DeleteUser(s.db, id); err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		_ = audit.Log(s.db, sess.AdminID, "user_delete", idStr, clientIP(r))
		s.requestReload()
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleUserInbounds — POST /users/{id}/inbounds
// Связывает пользователя с инбаундом, генерируя credential.
type attachPayload struct {
	InboundID      int64  `json:"inbound_id"`
	CredentialJSON string `json:"credential_json,omitempty"`
}

func (s *Server) handleUserInbounds(w http.ResponseWriter, r *http.Request) {
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

	// /users/{id}/inbounds
	path := strings.TrimPrefix(r.URL.Path, s.secret+"/users/")
	parts := strings.Split(path, "/")
	if len(parts) < 2 || parts[1] != "inbounds" {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}
	userID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		http.Error(w, "invalid user id", http.StatusBadRequest)
		return
	}

	var p attachPayload
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	cred := p.CredentialJSON
	if cred == "" {
		in, err := db.GetInbound(s.db, p.InboundID)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "inbound not found"})
			return
		}
		cred, err = generateCredential(in.Protocol)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
	}

	if err := db.AttachUserToInbound(s.db, db.UserInbound{
		UserID:         userID,
		InboundID:      p.InboundID,
		CredentialJSON: cred,
	}); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	_ = audit.Log(s.db, sess.AdminID, "user_inbound_attach", "", clientIP(r))
	s.requestReload()
	writeJSON(w, http.StatusOK, map[string]any{"credential_json": cred})
}

// generateCredential создаёт учётные данные для протокола.
func generateCredential(protocol string) (string, error) {
	switch protocol {
	case "vless", "vmess":
		uuid, err := randomUUID()
		if err != nil {
			return "", err
		}
		return `{"id":"` + uuid + `"}`, nil
	case "trojan", "shadowsocks":
		pw, err := randomPassword(16)
		if err != nil {
			return "", err
		}
		return `{"password":"` + pw + `"}`, nil
	default:
		return "{}", nil
	}
}

func randomUUID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	// RFC 4122 v4
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	s := hex.EncodeToString(b)
	return s[0:8] + "-" + s[8:12] + "-" + s[12:16] + "-" + s[16:20] + "-" + s[20:32], nil
}

func randomPassword(n int) (string, error) {
	const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	out := make([]byte, n)
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	for i := range out {
		out[i] = alphabet[int(buf[i])%len(alphabet)]
	}
	return string(out), nil
}
