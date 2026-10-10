// SPDX-License-Identifier: AGPL-3.0-or-later

package api

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
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
			"error": err.Error(),
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
