// SPDX-License-Identifier: AGPL-3.0-or-later

package api

import (
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
	Password string `json:"password"`
	Code     string `json:"code"`
}

type totpEnableResponse struct {
	OK            bool     `json:"ok"`
	RecoveryCodes []string `json:"recovery_codes,omitempty"`
	Error         string   `json:"error,omitempty"`
}

type totpSetupRequest struct {
	Password string `json:"password"`
}

// handleTOTPSetup — POST /2fa/setup
//
// Генерирует секрет и сохраняет его как pending. Требует текущий
// пароль — иначе украденная сессия может подменить pending-секрет
// на свой, а затем активировать его через /2fa/enable без ведома
// владельца. См. P0-2 второго аудита.
//
// Если 2FA уже включена — отказываем: ротация делается через
// disable (с паролем и текущим кодом) и повторный setup.
func (s *Server) handleTOTPSetup(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireSession(w, r)
	if !ok {
		return
	}
	if !s.requireCSRF(w, r, sess) {
		return
	}

	var req totpSetupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if req.Password == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "password required"})
		return
	}

	admin, err := auth.GetAdminByID(s.db, sess.AdminID)
	if err != nil {
		s.log.Error("get admin failed", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Setup сбрасывает счётчик: TOTP-кода здесь ещё нет (он только
	// генерируется). Иначе первый шаг настройки 2FA был бы невозможен
	// после случайной неудачи с паролем.
	if !s.verifyFreshPassword(w, r, sess, admin, req.Password, "totp_setup", true) {
		return
	}

	// Если 2FA уже включена — только явная ротация (disable → setup).
	if admin.TOTPSecret != "" {
		writeJSON(w, http.StatusConflict, map[string]string{
			"error": "2FA is already enabled, disable it first",
		})
		return
	}

	secret, url, err := auth.GenerateTOTPSecret(admin.Username)
	if err != nil {
		s.log.Error("generate totp failed", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if err := auth.SavePendingTOTPSecret(s.db, sess.AdminID, secret); err != nil {
		s.log.Error("save pending totp secret failed", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	_ = audit.Log(s.db, sess.AdminID, "totp_setup", "", s.clientIP(r))

	writeJSON(w, http.StatusOK, totpSetupResponse{
		Secret:     secret,
		OTPAuthURL: url,
	})
}

// handleTOTPQR — GET /2fa/qr
//
// Отдаёт PNG с QR-кодом для pending-секрета ЭТОГО админа.
// Секрет НЕ принимается из query (P3-5 третьего аудита):
// раньше URL otpauth://...secret=... летел в access-логи прокси,
// history браузера, Referer. Теперь читаем pending-секрет из БД
// и собираем otpauth URL сами.
func (s *Server) handleTOTPQR(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	pending, err := auth.GetPendingTOTPSecret(s.db, sess.AdminID)
	if err != nil {
		s.log.Error("get pending totp failed", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if pending == "" {
		http.Error(w, "setup not started", http.StatusBadRequest)
		return
	}

	admin, err := auth.GetAdminByID(s.db, sess.AdminID)
	if err != nil {
		s.log.Error("get admin failed", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	otpauthURL, err := auth.BuildOTPAuthURL(admin.Username, pending)
	if err != nil {
		s.log.Error("build otpauth url failed", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
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
//
// Требует текущий пароль и валидный код из pending-секрета.
// После активации отзывает все сессии, кроме текущей: если сессия
// украдена, атакующий не получит доступ после включения 2FA
// владельцем. См. P0-2 второго аудита.
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
	if req.Password == "" || req.Code == "" {
		writeJSON(w, http.StatusBadRequest, totpEnableResponse{
			Error: "password and code required",
		})
		return
	}

	admin, err := auth.GetAdminByID(s.db, sess.AdminID)
	if err != nil {
		s.log.Error("get admin failed", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// clearOnSuccess=false: после пароля проверяется TOTP-код.
	// Счётчик сбросим только после успешной проверки обоих
	// факторов. См. P1-4.
	if !s.verifyFreshPassword(w, r, sess, admin, req.Password, "totp_enable", false) {
		return
	}

	pending, err := auth.GetPendingTOTPSecret(s.db, sess.AdminID)
	if err != nil {
		s.log.Error("get pending totp failed", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if pending == "" {
		writeJSON(w, http.StatusBadRequest, totpEnableResponse{
			Error: "setup not started",
		})
		return
	}

	if !auth.VerifyTOTP(pending, req.Code) {
		_ = auth.RecordAdminFailedAttempt(s.db, admin.ID)
		_ = audit.Log(s.db, sess.AdminID, "totp_enable_failed", "code", s.clientIP(r))
		writeJSON(w, http.StatusUnauthorized, totpEnableResponse{
			Error: "invalid code",
		})
		return
	}

	// Оба фактора прошли — теперь можно сбросить счётчик.
	_ = auth.ClearAdminFailedAttempts(s.db, admin.ID)

	if err := auth.ActivatePendingTOTP(s.db, sess.AdminID); err != nil {
		if errors.Is(err, auth.ErrTOTPAlreadyEnabled) {
			writeJSON(w, http.StatusConflict, totpEnableResponse{
				Error: "2FA is already enabled",
			})
			return
		}
		s.log.Error("activate totp failed", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	codes, err := auth.GenerateRecoveryCodes(s.db, sess.AdminID)
	if err != nil {
		s.log.Error("generate recovery codes failed", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Отзываем все сессии, кроме текущей: даже если сессия украдена,
	// после включения 2FA старая сессия недействительна.
	_ = auth.DeleteOtherSessions(s.db, sess.AdminID, sess.ID)

	_ = audit.Log(s.db, sess.AdminID, audit.ActionTOTPEnabled, "", s.clientIP(r))

	writeJSON(w, http.StatusOK, totpEnableResponse{
		OK:            true,
		RecoveryCodes: codes,
	})
}

// handleTOTPDisable — POST /2fa/disable
type totpDisableRequest struct {
	Password string `json:"password"`
	Code     string `json:"totp_code"`
}

// handleTOTPDisable — POST /2fa/disable
//
// Требует пароль и валидный TOTP-код. Иначе украденная сессия
// отключает 2FA одним запросом — а 2FA нужна именно на случай
// кражи сессии. См. P0-2 второго аудита.
func (s *Server) handleTOTPDisable(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireSession(w, r)
	if !ok {
		return
	}
	if !s.requireCSRF(w, r, sess) {
		return
	}

	var req totpDisableRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if req.Password == "" || req.Code == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "password and totp_code required"})
		return
	}

	admin, err := auth.GetAdminByID(s.db, sess.AdminID)
	if err != nil {
		s.log.Error("get admin failed", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if admin.TOTPSecret == "" {
		// 2FA уже отключена — нечего проверять.
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "2FA is not enabled"})
		return
	}

	// clearOnSuccess=false: после пароля проверяется TOTP-код.
	// Счётчик сбросим только после успешной проверки обоих
	// факторов. См. P1-4.
	if !s.verifyFreshPassword(w, r, sess, admin, req.Password, "totp_disable", false) {
		return
	}
	if !auth.VerifyAndConsumeTOTP(s.db, admin.ID, admin.TOTPSecret, req.Code) {
		_ = auth.RecordAdminFailedAttempt(s.db, admin.ID)
		_ = audit.Log(s.db, sess.AdminID, "totp_disable_failed", "totp", s.clientIP(r))
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "invalid credentials"})
		return
	}

	// Оба фактора прошли — теперь можно сбросить счётчик.
	_ = auth.ClearAdminFailedAttempts(s.db, admin.ID)

	if err := auth.ClearTOTPSecret(s.db, sess.AdminID); err != nil {
		s.log.Error("clear totp failed", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	_ = audit.Log(s.db, sess.AdminID, audit.ActionTOTPDisabled, "", s.clientIP(r))

	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// handleTOTPStatus — GET /2fa/status
func (s *Server) handleTOTPStatus(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireSession(w, r)
	if !ok {
		return
	}
	admin, err := auth.GetAdminByID(s.db, sess.AdminID)
	if err != nil {
		s.log.Error("get admin failed", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{
		"enabled": admin.TOTPSecret != "",
	})
}
