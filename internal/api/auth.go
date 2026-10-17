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
	"strings"
	"time"

	"github.com/bassvii/gopanel/internal/audit"
	"github.com/bassvii/gopanel/internal/auth"
	"github.com/bassvii/gopanel/internal/notify"
)

const sessionCookieName = "gopanel_session"

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	TOTPCode string `json:"totp_code,omitempty"`
}

type loginResponse struct {
	OK        bool   `json:"ok"`
	CSRFToken string `json:"csrf_token,omitempty"`
	NeedsTOTP bool   `json:"needs_totp,omitempty"`
	Error     string `json:"error,omitempty"`
}

// handleLogin — POST /login
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ip := s.clientIP(r)

	// Для loopback/приватных IP (SSH-туннель, docker-bridge) не
	// применяем жёсткий lockout: все запросы приходят с одного
	// адреса, и атакующий мог бы заблокировать вход владельцу.
	// Вместо этого — задержка (P1-5).
	softLockout := auth.IsPrivateOrLoopback(ip) && len(s.trustedNets) == 0

	locked, wait, err := auth.IsLockedOut(s.db, ip)
	if err != nil {
		s.log.Error("lockout check failed", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if locked {
		if softLockout {
			// Не блокируем, но замедляем: чем больше неудач,
			// тем дольше ответ.
			delay := wait
			if delay > 6*time.Second {
				delay = 6 * time.Second
			}
			select {
			case <-r.Context().Done():
				return
			case <-time.After(delay):
			}
			_ = auth.ClearFailedAttempts(s.db, ip)
		} else {
			w.Header().Set("Retry-After", strconv.Itoa(int(wait.Seconds())))
			writeJSON(w, http.StatusTooManyRequests, loginResponse{Error: "too many attempts, try later"})
			return
		}
	}

	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, loginResponse{Error: "invalid json"})
		return
	}

	admin, err := auth.GetAdminByUsername(s.db, req.Username)
	if errors.Is(err, sql.ErrNoRows) {
		auth.VerifyPasswordDummy(req.Password)
		s.recordFailure(r, ip, 0)
		// Не пишем сам логин: пользователь мог ввести пароль
		// в поле "логин" случайно. См. P2-3.
		_ = audit.Log(s.db, 0, audit.ActionLoginFailed, "user:"+audit.HashLogin(req.Username), ip)
		writeJSON(w, http.StatusUnauthorized, loginResponse{Error: "invalid credentials"})
		return
	}
	if err != nil {
		s.log.Error("get admin failed", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Проверка lockout по админу.
	adminLocked, adminWait, err := auth.IsAdminLockedOut(s.db, admin.ID)
	if err != nil {
		s.log.Error("admin lockout check failed", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if adminLocked {
		// Исключение для владельца: если админ прислал валидный
		// TOTP-код ИЛИ неиспользованный recovery-код, lockout по
		// аккаунту обходится. Это защита от DoS (P1-9 первого
		// аудита): атакующий не может надолго заблокировать
		// владельца, зная только его логин.
		//
		// P3-1 третьего аудита: recovery-код тоже даёт обход.
		// Без этого владелец, потерявший телефон, оказывается
		// заблокирован, пока атака не прекратится.
		//
		// ВАЖНО: обход — это только «грубая» проверка валидности
		// без потребления. Настоящее потребление (ConsumeRecoveryCode
		// и VerifyAndConsumeTOTP) произойдёт ниже, в основном блоке,
		// ПОСЛЕ проверки пароля. Иначе перебор recovery-кодов при
		// lockout сжёг бы все коды.
		bypass := false
		if admin.TOTPSecret != "" && req.TOTPCode != "" {
			// TOTP-код.
			if auth.VerifyTOTP(admin.TOTPSecret, req.TOTPCode) {
				bypass = true
			} else if auth.RecoveryCodeExists(s.db, admin.ID, req.TOTPCode) {
				// Recovery-код — не потребляем, только проверяем наличие.
				bypass = true
			}
		}
		if !bypass {
			// Неудачная попытка обхода: продлеваем блокировку,
			// пишем в аудит, отдаём тот же ответ, что при неверном
			// пароле (чтобы не было оракула). См. P1-4 (новый аудит).
			s.recordFailure(r, ip, admin.ID)
			_ = audit.Log(s.db, admin.ID, audit.ActionLoginFailed, admin.Username+":lockout_bypass", ip)

			w.Header().Set("Retry-After", strconv.Itoa(int(adminWait.Seconds())))
			writeJSON(w, http.StatusTooManyRequests, loginResponse{Error: "invalid credentials"})
			return
		}
	}

	if err := auth.VerifyPassword(req.Password, admin.PasswordHash); err != nil {
		s.recordFailure(r, ip, admin.ID) // IP-счётчик + admin-счётчик
		notify.LoginFailed(s.db, s.log, admin.Username, ip)
		_ = audit.Log(s.db, admin.ID, audit.ActionLoginFailed, admin.Username, ip)
		writeJSON(w, http.StatusUnauthorized, loginResponse{Error: "invalid credentials"})
		return
	}

	// --- TOTP ---
	// НЕ сбрасываем счётчик после пароля! Только после полного успеха.
	totpOK := false
	if admin.TOTPSecret != "" {
		if req.TOTPCode == "" {
			// Не раскрываем валидность пароля: даже если он верный,
			// считаем это неудачной попыткой. Иначе атакующий по
			// разнице 200/401 понимает, что угадал пароль. См. #7.
			s.recordFailure(r, ip, admin.ID)
			writeJSON(w, http.StatusOK, loginResponse{
				NeedsTOTP: true,
			})
			return
		}
		// VerifyAndConsumeTOTP защищает от replay: один и тот же
		// код не пройдёт второй раз, даже если он ещё валиден по
		// времени. См. P1-8 второго аудита.
		if auth.VerifyAndConsumeTOTP(s.db, admin.ID, admin.TOTPSecret, req.TOTPCode) {
			totpOK = true
		} else {
			// Попробовать recovery-код.
			if err := auth.ConsumeRecoveryCode(s.db, admin.ID, req.TOTPCode); err == nil {
				totpOK = true
				_ = audit.Log(s.db, admin.ID, audit.ActionRecoveryUsed, "", ip)
			}
		}
		if !totpOK {
			s.recordFailure(r, ip, admin.ID)
			notify.LoginFailed(s.db, s.log, admin.Username, ip)
			_ = audit.Log(s.db, admin.ID, audit.ActionLoginFailed, admin.Username+":totp", ip)
			writeJSON(w, http.StatusUnauthorized, loginResponse{Error: "invalid code"})
			return
		}
	}

	// --- Успех ---
	// Только теперь сбрасываем счётчик.
	_ = auth.ClearFailedAttempts(s.db, ip)
	_ = auth.ClearAdminFailedAttempts(s.db, admin.ID)

	token, sess, err := auth.NewSession(s.db, admin.ID, ip, r.UserAgent())
	if err != nil {
		s.log.Error("create session failed", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	_ = audit.Log(s.db, admin.ID, audit.ActionLogin, admin.Username, ip)
	notify.LoginSuccess(s.db, s.log, admin.Username, ip)

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     s.secret,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Secure:   s.cfg.SecureCookies,
		MaxAge:   int(auth.SessionTTL.Seconds()),
	})

	writeJSON(w, http.StatusOK, loginResponse{OK: true, CSRFToken: sess.CSRFToken})
}

// recordFailure обновляет счётчики неудач и, если softLockout,
// добавляет задержку, чтобы перебор был медленным.
func (s *Server) recordFailure(r *http.Request, ip string, adminID int64) {
	_ = auth.RecordFailedAttempt(s.db, ip)
	if adminID > 0 {
		_ = auth.RecordAdminFailedAttempt(s.db, adminID)
	}
	if auth.IsPrivateOrLoopback(ip) && len(s.trustedNets) == 0 {
		// Задержка 2 секунды. Точное число неудач знает только
		// БД, но 2 секунды на попытку уже замедляют перебор
		// в 10+ раз.
		select {
		case <-r.Context().Done():
		case <-time.After(2 * time.Second):
		}
	}
}

// handleLogout — POST /logout
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Если сессия валидна — требуем CSRF. Если cookie нет или
	// сессия истекла — просто чистим cookie, это безопасно
	// (нечего завершать).
	cookie, err := r.Cookie(sessionCookieName)
	if err == nil {
		if sess, err := auth.GetSession(s.db, cookie.Value); err == nil {
			if !s.requireCSRF(w, r, sess) {
				return
			}
			_ = auth.DeleteSession(s.db, sess.ID)
			_ = audit.Log(s.db, sess.AdminID, audit.ActionLogout, "", s.clientIP(r))
		}
	}

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     s.secret,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Secure:   s.cfg.SecureCookies,
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
//
// P3-4 третьего аудита: если IP или User-Agent сессии изменились,
// пишем в audit_log. Сессию НЕ рвём молча — мобильные сети
// меняют IP легитимно. При bind_session_ip=true — рвём.
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

	// Проверяем, что контекст сессии не изменился.
	curIP := s.clientIP(r)
	curUA := r.UserAgent()

	if sess.IP != curIP || sess.UserAgent != curUA {
		// Логируем один раз при смене (не на каждый запрос).
		_ = audit.Log(s.db, sess.AdminID, "session_context_changed", sess.ID, curIP)

		// Обновляем запись, чтобы следующая смена снова залогировалась.
		if err := auth.UpdateSessionContext(s.db, sess.ID, curIP, curUA); err != nil {
			s.log.Error("update session context failed", "err", err)
		}
		sess.IP = curIP
		sess.UserAgent = curUA
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

// clientIP возвращает IP клиента для логов, rate limit и audit.
//
// Если RemoteAddr входит в trustedNets (доверенные прокси), берём
// адрес из X-Forwarded-For — последний недоверенный в цепочке
// (стандартный подход, чтобы клиент не мог подделать IP, дописав
// свой XFF-заголовок).
//
// Иначе (RemoteAddr не доверенный) — возвращаем RemoteAddr:
// заголовок X-Forwarded-For игнорируется, подмена невозможна.
func (s *Server) clientIP(r *http.Request) string {
	remoteIP := extractHost(r.RemoteAddr)

	if !s.isTrustedProxy(remoteIP) {
		return remoteIP
	}

	xff := r.Header.Get("X-Forwarded-For")
	if xff == "" {
		return remoteIP
	}

	// XFF: "client, proxy1, proxy2". Идём справа налево, пропуская
	// доверенные адреса. Первый недоверенный — клиент.
	parts := strings.Split(xff, ",")
	for i := len(parts) - 1; i >= 0; i-- {
		ip := strings.TrimSpace(parts[i])
		if ip == "" {
			continue
		}
		if !s.isTrustedProxy(ip) {
			return ip
		}
	}

	// Все адреса в XFF доверенные — берём самый левый.
	return strings.TrimSpace(parts[0])
}

// isTrustedProxy проверяет, входит ли IP в trustedNets.
func (s *Server) isTrustedProxy(ipStr string) bool {
	if len(s.trustedNets) == 0 {
		return false
	}
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}
	for _, n := range s.trustedNets {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

// extractHost возвращает хост из "host:port" или сам addr,
// если разделителя нет.
func extractHost(addr string) string {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	return host
}
