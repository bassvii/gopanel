// SPDX-License-Identifier: AGPL-3.0-or-later

package api

import (
	"net/http"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"
)

// withMiddleware оборачивает mux в цепочку middleware.
// maxRequestBody — лимит на размер тела запроса. 1 МБ хватает
// для любого нашего JSON (пользователи, инбаунды, настройки),
// но не даёт клиенту выесть память гигабайтом.
const maxRequestBody = 1 << 20 // 1 MiB

// withMiddleware оборачивает mux в цепочку middleware.
func (s *Server) withMiddleware(next http.Handler) http.Handler {
	return s.recoverer(s.logger(s.securityHeaders(s.limitBody(s.spaFallback(next)))))
}

// limitBody ограничивает размер тела запроса. Для GET/DELETE
// лимит не важен — там нет тела. Для POST/PUT — обрезает всё,
// что больше maxRequestBody, и хендлер получит ошибку при чтении.
func (s *Server) limitBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body != nil {
			r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)
		}
		next.ServeHTTP(w, r)
	})
}

// spaFallback отдаёт index.html для навигации браузера на роуты SPA.
//
// Без этого:
//   - GET /<base>/users  → API вернёт JSON (это ломает прямую навигацию)
//   - GET /<base>/users/ → handleStatic отдаст index.html (работает)
//
// Здесь мы смотрим Sec-Fetch-Dest: document — этот заголовок браузер
// ставит только при загрузке страницы, но не при fetch/XHR. Если путь
// под base_path и это навигация — отдаём index.html. См. P1-2.
func (s *Server) spaFallback(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Только GET и только для навигации браузера.
		if r.Method == http.MethodGet &&
			r.Header.Get("Sec-Fetch-Dest") == "document" &&
			strings.HasPrefix(r.URL.Path, s.secret+"/") &&
			s.webFS != nil {

			// Путь под base_path. Если это API-эндпоинт — не
			// перехватываем. Но навигация под /base/users/ —
			// это SPA-роут, а не API (API — /base/users без слэша).
			trimmed := strings.TrimPrefix(r.URL.Path, s.secret)
			trimmed = strings.TrimSuffix(trimmed, "/")

			// Известные API-роуты (без слэша на конце).
			// Если браузер открывает их напрямую — покажем SPA,
			// а не JSON (пользователь всё равно не сможет прочитать JSON).
			switch trimmed {
			case "/users", "/inbounds", "/outbounds", "/routing/rules",
				"/audit", "/notifications", "/me", "/me/sessions",
				"/stats/summary", "/system/stats", "/system/geo/status":
				// Это API — но пользователь открыл в браузере.
				// Отдаём SPA, чтобы он мог перейти по навигации.
			}

			// Убеждаемся, что это не запрос к ассету.
			if filepath.Ext(r.URL.Path) == "" {
				s.serveIndex(w, r)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// recoverer ловит паники в обработчиках и отвечает 500.
func (s *Server) recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				s.log.Error("panic in handler",
					"err", rec,
					"path", r.URL.Path,
					"stack", string(debug.Stack()),
				)
				http.Error(w, "internal server error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// logger пишет строку на каждый запрос.
func (s *Server) logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rw, r)
		s.log.Info("http",
			"method", r.Method,
			"path", s.sanitizePath(r.URL.Path),
			"status", rw.status,
			"duration_ms", time.Since(start).Milliseconds(),
			"remote", s.clientIP(r),
		)
	})
}

// sanitizePath убирает секретный base path из пути, чтобы он
// не попадал в логи. В логе остаётся /<endpoint>, а не /<hex>/<endpoint>.
func (s *Server) sanitizePath(path string) string {
	if s.secret != "" && strings.HasPrefix(path, s.secret) {
		rest := strings.TrimPrefix(path, s.secret)
		if rest == "" {
			return "/"
		}
		return rest
	}
	return path
}

// securityHeaders выставляет базовые заголовки безопасности.
func (s *Server) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Content-Security-Policy",
			"default-src 'self'; "+
				"img-src 'self' data:; "+
				"style-src 'self' 'unsafe-inline'; "+
				"script-src 'self'; "+
				"connect-src 'self'; "+
				"frame-ancestors 'none'")
		h.Set("Cross-Origin-Opener-Policy", "same-origin")
		next.ServeHTTP(w, r)
	})
}

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}
