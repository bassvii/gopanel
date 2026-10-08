// SPDX-License-Identifier: AGPL-3.0-or-later

package api

import (
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"errors"
	"net"
	"net/http"
    "strconv"

	"github.com/bassvii/gopanel/internal/auth"
	"github.com/bassvii/gopanel/internal/audit"
)

const sessionCookieName = "gopanel_session"

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	TOTPCode string `json:"totp_code,omitempty"`
}

type loginResponse struct {
	OK       bool   `json:"ok"`
	CSRFToken string `json:"csrf_token,omitempty"`
	NeedsTOTP bool   `json:"needs_totp,omitempty"`
	Error    string `json:"error,omitempty"`
}

// handleLogin — POST /login
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ip := clientIP(r)

	locked, wait, err := auth.IsLockedOut(s.db, ip)
	if err != nil {
		s.log.Error("lockout check failed", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if locked {
		w.Header().Set("Retry-After", strconv.Itoa(int(wait.Seconds())))
		writeJSON(w, http.StatusTooManyRequests, loginResponse{Error: "too many attempts, try later"})
		return
	}

	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, loginResponse{Error: "invalid json"})
		return
	}

	admin, err := auth.GetAdminByUsername(s.db, req.Username)
	if errors.Is(err, sql.ErrNoRows) {
		_ = auth.RecordFailedAttempt(s.db, ip)
		writeJSON(w, http.StatusUnauthorized, loginResponse{Error: "invalid credentials"})
		return
	}
	if err != nil {
		s.log.Error("get admin failed", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if err := auth.VerifyPassword(req.Password, admin.PasswordHash); err != nil {
		_ = auth.RecordFailedAttempt(s.db, ip)
		_ = audit.Log(s.db, admin.ID, audit.ActionLoginFailed, "wrong password", ip)
		writeJSON(w, http.StatusUnauthorized, loginResponse{Error: "invalid credentials"})
		return
	}

	_ = auth.ClearFailedAttempts(s.db, ip)

	// Если у админа включена 2FA — требуем код.
	if admin.TOTPSecret != "" {
		if req.TOTPCode == "" {
			writeJSON(w, http.StatusUnauthorized, loginResponse{
				Error:    "totp_required",
				NeedsTOTP: true,
			})
			return
		}
		if !auth.VerifyTOTP(admin.TOTPSecret, req.TOTPCode) {
			if err := auth.ConsumeRecoveryCode(s.db, admin.ID, req.TOTPCode); err != nil {
				_ = auth.RecordFailedAttempt(s.db, ip)
				_ = audit.Log(s.db, admin.ID, audit.ActionLoginFailed, "wrong totp", ip)
				writeJSON(w, http.StatusUnauthorized, loginResponse{Error: "invalid code"})
				return
			}
			_ = audit.Log(s.db, admin.ID, audit.ActionRecoveryUsed, "", ip)
		}

	}

	token, sess, err := auth.NewSession(s.db, admin.ID, ip, r.UserAgent())
	if err != nil {
		s.log.Error("create session failed", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	_ = audit.Log(s.db, admin.ID, audit.ActionLogin, admin.Username, ip)

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     s.secret,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Secure:   false, // админка на loopback, HTTPS не нужен
		MaxAge:   int(auth.SessionTTL.Seconds()),
	})

	writeJSON(w, http.StatusOK, loginResponse{OK: true, CSRFToken: sess.CSRFToken})
}

// handleLogout — POST /logout
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	cookie, err := r.Cookie(sessionCookieName)
	if err == nil {
		if sess, err := auth.GetSession(s.db, cookie.Value); err == nil {
			_ = auth.DeleteSession(s.db, sess.ID)
			_ = audit.Log(s.db, sess.AdminID, audit.ActionLogout, "", clientIP(r))
		}
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     s.secret,
		HttpOnly: true,
		MaxAge:   -1,
	})
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// handleMe — GET /me
func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireSession(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"admin_id":   sess.AdminID,
		"csrf_token": sess.CSRFToken,
	})
}

// requireSession проверяет cookie и возвращает сессию.
// При ошибке пишет 401 и возвращает false.
func (s *Server) requireSession(w http.ResponseWriter, r *http.Request) (auth.Session, bool) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return auth.Session{}, false
	}
	sess, err := auth.GetSession(s.db, cookie.Value)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return auth.Session{}, false
	}
	_ = auth.TouchSession(s.db, sess.ID)
	return sess, true
}

// requireCSRF проверяет заголовок X-CSRF-Token против сессии.
func (s *Server) requireCSRF(w http.ResponseWriter, r *http.Request, sess auth.Session) bool {
	got := r.Header.Get("X-CSRF-Token")
	if got == "" || subtle.ConstantTimeCompare([]byte(got), []byte(sess.CSRFToken)) != 1 {
		http.Error(w, "invalid csrf token", http.StatusForbidden)
		return false
	}
	return true
}

var _ = (*Server).requireCSRF

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// clientIP вытаскивает IP из RemoteAddr.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
