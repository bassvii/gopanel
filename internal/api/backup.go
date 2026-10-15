// SPDX-License-Identifier: AGPL-3.0-or-later

package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/bassvii/gopanel/internal/audit"
	"github.com/bassvii/gopanel/internal/auth"
	"github.com/bassvii/gopanel/internal/backup"
	"github.com/bassvii/gopanel/internal/notify"
)

type backupRequest struct {
	Password string `json:"password"`
}

// handleBackupDownload — POST /backup/download
//
// Требует пароль администратора в теле запроса. Бэкап содержит
// хеши паролей, TOTP-секреты (зашифрованные), recovery-коды —
// украденная сессия не должна давать доступ ко всему этому без
// знания пароля (см. P0-2 аудита).
func (s *Server) handleBackupDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	sess, ok := s.requireSession(w, r)
	if !ok {
		return
	}
	if !s.requireCSRF(w, r, sess) {
		return
	}

	var req backupRequest
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

	if !s.verifyFreshPassword(w, r, sess, admin, req.Password, "backup_download") {
		return
	}

	name := backup.BackupName()

	// Большая база может отдаваться дольше 30 секунд — продлеваем
	// дедлайн записи только для этой ручки.
	if rc := http.NewResponseController(w); rc != nil {
		_ = rc.SetWriteDeadline(time.Now().Add(6 * time.Minute))
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	w.Header().Set("Cache-Control", "no-store")

	if err := backup.DumpToWriter(r.Context(), s.db, w); err != nil {
		s.log.Error("backup dump failed", "err", err)
		// Заголовки уже отправлены — просто прекращаем.
		return
	}

	_ = audit.Log(s.db, sess.AdminID, "backup_download", name, s.clientIP(r))
	notify.BackupCreated(s.db, s.log, name)
}
