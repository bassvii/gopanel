// SPDX-License-Identifier: AGPL-3.0-or-later

package api

import (
	"net/http"

	"github.com/bassvii/gopanel/internal/audit"
	"github.com/bassvii/gopanel/internal/backup"
)

// handleBackupDownload — GET /backup/download
func (s *Server) handleBackupDownload(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	name := backup.BackupName()

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	w.Header().Set("Cache-Control", "no-store")

	if err := backup.DumpToWriter(r.Context(), s.db, w); err != nil {
		s.log.Error("backup dump failed", "err", err)
		// Заголовки уже отправлены — просто прекращаем.
		return
	}

	_ = audit.Log(s.db, sess.AdminID, "backup_download", name, clientIP(r))
}
