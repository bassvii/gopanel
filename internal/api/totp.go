// SPDX-License-Identifier: AGPL-3.0-or-later

package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/bassvii/gopanel/internal/audit"
	"github.com/bassvii/gopanel/internal/auth"
	qrcode "github.com/skip2/go-qrcode"
)

type totpSetupResponse struct {
	Secret     string `json:"secret"`
	OTPAuthURL string `json:"otpauth_url"`
}

type totpEnableRequest struct {
	Code string `json:"code"`
}

type totpEnableResponse struct {
	OK            bool     `json:"ok"`
	RecoveryCodes []string `json:"recovery_codes,omitempty"`
	Error         string   `json:"error,omitempty"`
}

// handleTOTPSetup — POST /2fa/setup
// Генерирует секрет и возвращает его с otpauth-URL, но НЕ сохраняет.
// Сохранение — на шаге enable, после проверки кода.
func (s *Server) handleTOTPSetup(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireSession(w, r)
	if !ok {
		return
	}
	if !s.requireCSRF(w, r, sess) {
		return
	}

	admin, err := getAdminByID(s.db, sess.AdminID)
	if err != nil {
		s.log.Error("get admin failed", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	secret, url, err := auth.GenerateTOTPSecret(admin.Username)
	if err != nil {
		s.log.Error("generate totp failed", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if err := auth.SaveTOTPSecret(s.db, sess.AdminID, secret); err != nil {
		s.log.Error("save totp secret failed", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, totpSetupResponse{
		Secret:     secret,
		OTPAuthURL: url,
	})
}

// handleTOTPQR — GET /2fa/qr?url=<otpauth-url>
// Отдаёт PNG с QR-кодом. URL передаётся в query, чтобы не хранить состояние.
func (s *Server) handleTOTPQR(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireSession(w, r); !ok {
		return
	}

	otpauthURL := r.URL.Query().Get("url")
	if otpauthURL == "" {
		http.Error(w, "missing url", http.StatusBadRequest)
		return
	}

	png, err := qrcode.Encode(otpauthURL, qrcode.Medium, 256)
	if err != nil {
		s.log.Error("qrcode encode failed", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(png)
}

// handleTOTPEnable — POST /2fa/enable
// Проверяет код, сохраняет секрет, генерирует коды восстановления.
func (s *Server) handleTOTPEnable(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireSession(w, r)
	if !ok {
		return
	}
	if !s.requireCSRF(w, r, sess) {
		return
	}

	var req totpEnableRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, totpEnableResponse{Error: "invalid json"})
		return
	}

	admin, err := getAdminByID(s.db, sess.AdminID)
	if err != nil {
		s.log.Error("get admin failed", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if !auth.VerifyTOTP(admin.TOTPSecret, req.Code) {
		writeJSON(w, http.StatusUnauthorized, totpEnableResponse{
			Error: "invalid code",
		})
		return
	}

	codes, err := auth.GenerateRecoveryCodes(s.db, sess.AdminID)
	if err != nil {
		s.log.Error("generate recovery codes failed", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	_ = audit.Log(s.db, sess.AdminID, audit.ActionTOTPEnabled, "", clientIP(r))

	writeJSON(w, http.StatusOK, totpEnableResponse{
		OK:            true,
		RecoveryCodes: codes,
	})
}

// handleTOTPDisable — POST /2fa/disable
func (s *Server) handleTOTPDisable(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireSession(w, r)
	if !ok {
		return
	}
	if !s.requireCSRF(w, r, sess) {
		return
	}

	if err := auth.ClearTOTPSecret(s.db, sess.AdminID); err != nil {
		s.log.Error("clear totp failed", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	_ = audit.Log(s.db, sess.AdminID, audit.ActionTOTPDisabled, "", clientIP(r))

	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// handleTOTPStatus — GET /2fa/status
func (s *Server) handleTOTPStatus(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireSession(w, r)
	if !ok {
		return
	}
	admin, err := getAdminByID(s.db, sess.AdminID)
	if err != nil {
		s.log.Error("get admin failed", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{
		"enabled": admin.TOTPSecret != "",
	})
}

// getAdminByID — вспомогательная: получить админа по ID.
func getAdminByID(conn *sql.DB, id int64) (auth.Admin, error) {
	var a auth.Admin
	err := conn.QueryRow(
		`SELECT id, username, password_hash, totp_secret, role
		 FROM admins WHERE id = ?`,
		id,
	).Scan(&a.ID, &a.Username, &a.PasswordHash, &a.TOTPSecret, &a.Role)
	if errors.Is(err, sql.ErrNoRows) {
		return a, sql.ErrNoRows
	}
	return a, err
}
