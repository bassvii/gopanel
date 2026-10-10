// SPDX-License-Identifier: AGPL-3.0-or-later

// Package backup создаёт дампы SQLite-базы.
package backup

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// Dump создаёт логический дамп базы в файл через VACUUM INTO.
// Это безопасно для работающей базы: SQLite берёт консистентный снапшот.
func Dump(ctx context.Context, conn *sql.DB, outPath string) error {
	if err := os.MkdirAll(filepath.Dir(outPath), 0o700); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	// VACUUM INTO создаёт сжатый слепок базы.
	// Файл должен не существовать.
	_ = os.Remove(outPath)

	query := fmt.Sprintf(`VACUUM INTO '%s'`, escapeSQLString(outPath))
	if _, err := conn.ExecContext(ctx, query); err != nil {
		return fmt.Errorf("vacuum into: %w", err)
	}
	if err := os.Chmod(outPath, 0o600); err != nil {
		return fmt.Errorf("chmod: %w", err)
	}
	return nil
}

// DumpToWriter создаёт дамп во временный файл и отдаёт его в w.
func DumpToWriter(ctx context.Context, conn *sql.DB, w io.Writer) error {
	tmp, err := os.CreateTemp("", "gopanel-backup-*.db")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	tmp.Close()
	defer os.Remove(tmpPath)

	if err := Dump(ctx, conn, tmpPath); err != nil {
		return err
	}

	f, err := os.Open(tmpPath)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(w, f)
	return err
}

// BackupName возвращает имя файла бэкапа с датой.
func BackupName() string {
	return fmt.Sprintf("gopanel-%s.db", time.Now().Format("2006-01-02-150405"))
}

func escapeSQLString(s string) string {
	out := make([]byte, 0, len(s)+2)
	for i := 0; i < len(s); i++ {
		if s[i] == '\'' {
			out = append(out, '\'')
		}
		out = append(out, s[i])
	}
	return string(out)
}
