// SPDX-License-Identifier: AGPL-3.0-or-later

// Package backup создаёт дампы SQLite-базы.
package backup

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
	// Создаём каталог 0700 и файл 0600 сразу, чтобы не было окна,
	// когда файл с секретами читается другими пользователями.
	dir, err := os.MkdirTemp("", "gopanel-backup-*")
	if err != nil {
		return err
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		os.RemoveAll(dir)
		return err
	}
	defer os.RemoveAll(dir)

	tmpPath := filepath.Join(dir, "backup.db")
	tmp, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	tmp.Close()

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

// EncryptFile шифрует файл src в dst с помощью утилиты age.
// Пароль передаётся через stdin (не через аргументы, чтобы не
// попасть в ps). Файл src удаляется после успешного шифрования.
//
// Требует установленного в системе `age` (apt install age или
// brew install age). Если age не найден — возвращает понятную ошибку.
func EncryptFile(ctx context.Context, src, dst, password string) error {
	// Проверяем наличие age.
	if _, err := exec.LookPath("age"); err != nil {
		return fmt.Errorf("утилита age не найдена. Установите её: apt install age (или brew install age)")
	}

	// Открываем исходный файл.
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open source: %w", err)
	}
	defer in.Close()

	// Создаём целевой файл с правами 0600.
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("create target: %w", err)
	}
	defer out.Close()

	// age -e -p (passphrase mode) -o dst.
	// Пароль идёт через stdin (второй \n нужен для подтверждения).
	cmd := exec.CommandContext(ctx, "age", "-e", "-p", "-o", dst)
	cmd.Stdin = strings.NewReader(password + "\n" + password + "\n")
	cmd.Stdout = os.Stderr // прогресс/ошибки age идут в stderr
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		_ = out.Close()
		_ = os.Remove(dst)
		return fmt.Errorf("age encryption failed: %w", err)
	}

	// Удаляем исходный файл с секретами.
	if err := os.Remove(src); err != nil {
		return fmt.Errorf("remove plaintext backup: %w", err)
	}

	return nil
}
