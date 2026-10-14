// SPDX-License-Identifier: AGPL-3.0-or-later

package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"path/filepath"
	"regexp"

	"github.com/bassvii/gopanel/internal/audit"
)

// handleX25519 — POST /xray/x25519
// Возвращает пару ключей для REALITY.
func (s *Server) handleX25519(w http.ResponseWriter, r *http.Request) {
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

	if s.core == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "xray not available",
		})
		return
	}

	pair, err := s.core.GenerateX25519(r.Context())
	if err != nil {
		s.log.Error("x25519 generation failed", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "internal error",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"private_key": pair.PrivateKey,
		"public_key":  pair.PublicKey,
	})
}

// handleShortID — POST /xray/shortid
// Возвращает случайный shortId для REALITY.
func (s *Server) handleShortID(w http.ResponseWriter, r *http.Request) {
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

	// shortId — 8 байт hex (16 символов). Xray допускает до 16 hex-символов.
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"short_id": hex.EncodeToString(buf),
	})
}

// handleVlessEnc — POST /xray/vlessenc
func (s *Server) handleVlessEnc(w http.ResponseWriter, r *http.Request) {
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

	if s.core == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "xray not available",
		})
		return
	}

	res, err := s.core.GenerateVlessEnc()
	if err != nil {
		s.log.Error("vlessenc generation failed", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "internal error",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"server": res.Server,
		"client": res.Client,
	})
}

var (
	certNameRe   = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
	certDomainRe = regexp.MustCompile(`^(\*\.)?[A-Za-z0-9]([A-Za-z0-9-]{0,61}[A-Za-z0-9])?(\.[A-Za-z0-9]([A-Za-z0-9-]{0,61}[A-Za-z0-9])?)*$`)
)

type tlsCertRequest struct {
	Domain      string `json:"domain"`
	Name        string `json:"name"`
	ExpireHours int    `json:"expire_hours"`
}

type tlsCertResponse struct {
	CertPath string `json:"cert_path"`
	KeyPath  string `json:"key_path"`
	Domain   string `json:"domain"`
}

// handleTLSCert — POST /xray/tls-cert
// Body: {"domain": "example.com", "name": "hysteria", "expire_hours": 8760}
func (s *Server) handleTLSCert(w http.ResponseWriter, r *http.Request) {
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

	if s.core == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "xray not available",
		})
		return
	}

	var req tlsCertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	// Каталог для сертификатов рядом с БД.
	dir := "./certs"
	if s.dbPath != "" {
		dir = filepath.Join(filepath.Dir(s.dbPath), "certs")
	}

	name := req.Name
	if name == "" {
		name = "cert"
	}

	// Валидация до вызова ядра — чтобы отсеять мусор и traversal.
	if !certNameRe.MatchString(name) {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "invalid cert name: allowed a-z, A-Z, 0-9, _, - (1-64 chars)",
		})
		return
	}
	if req.Domain != "" && req.Domain != "localhost" && !certDomainRe.MatchString(req.Domain) {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "invalid domain",
		})
		return
	}
	if req.ExpireHours < 0 || req.ExpireHours > 87600 { // 10 лет максимум
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "expire_hours must be 0..87600",
		})
		return
	}

	res, err := s.core.GenerateTLSCert(dir, name, req.Domain, req.ExpireHours)
	if err != nil {
		s.log.Error("tls cert generation failed", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "internal error",
		})
		return
	}

	_ = audit.Log(s.db, sess.AdminID, "tls_cert_create", req.Domain, s.clientIP(r))
	writeJSON(w, http.StatusOK, tlsCertResponse{
		CertPath: res.CertPath,
		KeyPath:  res.KeyPath,
		Domain:   res.Domain,
	})
}
