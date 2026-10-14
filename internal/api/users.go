// SPDX-License-Identifier: AGPL-3.0-or-later

package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/mail"
	"strconv"
	"strings"
	"time"

	"github.com/bassvii/gopanel/internal/audit"
	"github.com/bassvii/gopanel/internal/db"
)

// userDTO — представление пользователя для API.
type userDTO struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Email       string `json:"email"`
	Enabled     bool   `json:"enabled"`
	QuotaBytes  int64  `json:"quota_bytes"`
	UsedBytes   int64  `json:"used_bytes"`
	ExpiresAt   int64  `json:"expires_at"`
	DeviceLimit int    `json:"device_limit"`
	Note        string `json:"note"`
	CreatedAt   int64  `json:"created_at"`
}

func toUserDTO(u db.User) userDTO {
	var expires, created int64
	if !u.ExpiresAt.IsZero() {
		expires = u.ExpiresAt.Unix()
	}
	if !u.CreatedAt.IsZero() {
		created = u.CreatedAt.Unix()
	}
	return userDTO{
		ID:          u.ID,
		Name:        u.Name,
		Email:       u.Email,
		Enabled:     u.Enabled,
		QuotaBytes:  u.QuotaBytes,
		UsedBytes:   u.UsedBytes,
		ExpiresAt:   expires,
		DeviceLimit: u.DeviceLimit,
		Note:        u.Note,
		CreatedAt:   created,
	}
}

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
		dtos := make([]userDTO, 0, len(list))
		for _, u := range list {
			dtos = append(dtos, toUserDTO(u))
		}
		writeJSON(w, http.StatusOK, map[string]any{"users": dtos})

	case http.MethodPost:
		if !s.requireCSRF(w, r, sess) {
			return
		}
		var p userPayload
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
			return
		}
		if err := validateUserPayload(p); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
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
		_ = audit.Log(s.db, sess.AdminID, "user_create", p.Name, s.clientIP(r))
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

	idStr := r.PathValue("id")
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
		writeJSON(w, http.StatusOK, toUserDTO(u))

	case http.MethodPut:
		if !s.requireCSRF(w, r, sess) {
			return
		}
		var p userPayload
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
			return
		}
		if err := validateUserPayload(p); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
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
		_ = audit.Log(s.db, sess.AdminID, "user_update", p.Name, s.clientIP(r))
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
		_ = audit.Log(s.db, sess.AdminID, "user_delete", idStr, s.clientIP(r))
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
	userIDStr := r.PathValue("id")
	userID, err := strconv.ParseInt(userIDStr, 10, 64)
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
	in, err := db.GetInbound(s.db, p.InboundID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "inbound not found"})
		return
	}

	if cred == "" {
		cred, err = generateCredential(in.Protocol)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
	} else {
		// Валидация присланного credential по протоколу.
		if err := validateCredential(in.Protocol, cred); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
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

	_ = audit.Log(s.db, sess.AdminID, "user_inbound_attach", "", s.clientIP(r))
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

// validateCredential проверяет, что JSON-строка credential подходит
// для данного протокола: содержит нужные поля правильного типа.
// См. P0-4 второго аудита.
func validateCredential(protocol, cred string) error {
	if cred == "" {
		return errors.New("credential is empty")
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(cred), &obj); err != nil {
		return fmt.Errorf("credential is not a JSON object: %w", err)
	}
	switch protocol {
	case "vless", "vmess":
		id, ok := obj["id"].(string)
		if !ok || id == "" {
			return errors.New("credential for vless/vmess must contain non-empty \"id\" (string)")
		}
	case "trojan", "shadowsocks":
		pw, ok := obj["password"].(string)
		if !ok || pw == "" {
			return errors.New("credential for trojan/shadowsocks must contain non-empty \"password\" (string)")
		}
	case "socks", "http":
		// user/password опциональны (может быть без авторизации),
		// но если задано — должен быть string.
		if v, ok := obj["user"]; ok {
			if _, isStr := v.(string); !isStr {
				return errors.New("credential \"user\" must be a string")
			}
		}
		if v, ok := obj["password"]; ok {
			if _, isStr := v.(string); !isStr {
				return errors.New("credential \"password\" must be a string")
			}
		}
	}
	return nil
}

// validateUserPayload проверяет поля пользователя для POST и PUT.
// См. P2-14 второго аудита.
func validateUserPayload(p userPayload) error {
	name := strings.TrimSpace(p.Name)
	if name == "" {
		return errors.New("name is required")
	}
	if len(name) > 64 {
		return errors.New("name must be at most 64 characters")
	}
	email := strings.TrimSpace(p.Email)
	if email != "" {
		if len(email) > 254 {
			return errors.New("email must be at most 254 characters")
		}
		if _, err := mail.ParseAddress(email); err != nil {
			return fmt.Errorf("invalid email: %w", err)
		}
	}
	if len(p.Note) > 1024 {
		return errors.New("note must be at most 1024 characters")
	}
	if p.QuotaBytes < 0 {
		return errors.New("quota_bytes must be non-negative")
	}
	if p.DeviceLimit < 0 {
		return errors.New("device_limit must be non-negative")
	}
	if p.ExpiresAt < 0 {
		return errors.New("expires_at must be non-negative")
	}
	return nil
}
