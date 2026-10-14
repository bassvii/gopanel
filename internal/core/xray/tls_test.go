// SPDX-License-Identifier: AGPL-3.0-or-later

package xray

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestGenerateTLSCert_RejectsPathTraversal проверяет, что имя с ".."
// или разделителями не выводит файлы за пределы каталога.
func TestGenerateTLSCert_RejectsPathTraversal(t *testing.T) {
	core := &Core{binPath: "/bin/false"} // генерация не запустится, но валидация — до неё

	dir := t.TempDir()

	cases := []string{
		"../escape",
		"../../etc/passwd",
		"foo/bar",
		"foo\\bar",
		"..",
		".",
		"",
		strings.Repeat("a", 65), // слишком длинное
		"name with spaces",
		"name.with.dots",
		"name;semicolon",
	}

	for _, name := range cases {
		_, err := core.GenerateTLSCert(dir, name, "example.com", 1)
		if err == nil {
			t.Errorf("name %q: expected error, got nil", name)
		}
	}
}

// TestGenerateTLSCert_RejectsBadDomain проверяет, что домен с мусором
// (пробелы, спецсимволы) не проходит.
func TestGenerateTLSCert_RejectsBadDomain(t *testing.T) {
	core := &Core{binPath: "/bin/false"}
	dir := t.TempDir()

	cases := []string{
		"example.com; rm -rf /",
		"domain with space",
		"-leading-hyphen.com",
		"trailing-hyphen-.com",
		"double..dot.com",
		"корм.рф", // не ASCII
	}

	for _, d := range cases {
		_, err := core.GenerateTLSCert(dir, "cert", d, 1)
		if err == nil {
			t.Errorf("domain %q: expected error, got nil", d)
		}
	}
}

// TestGenerateTLSCert_AcceptsValid проверяет, что валидные данные не падают
// на этапе валидации. Генерация через xray упадёт (binPath=/bin/false),
// поэтому проверяем, что ошибка — про xray, а не про валидацию.
func TestGenerateTLSCert_AcceptsValid(t *testing.T) {
	core := &Core{binPath: "/bin/false"}
	dir := t.TempDir()

	validNames := []string{"cert", "my-cert", "my_cert", "Cert123", "a"}
	for _, name := range validNames {
		_, err := core.GenerateTLSCert(dir, name, "example.com", 1)
		if err == nil {
			continue // маловероятно, /bin/false должен упасть
		}
		if strings.Contains(err.Error(), "invalid cert name") ||
			strings.Contains(err.Error(), "invalid domain") ||
			strings.Contains(err.Error(), "escapes directory") {
			t.Errorf("name %q: got validation error, expected xray error: %v", name, err)
		}
	}

	validDomains := []string{"example.com", "sub.example.com", "*.example.com", "localhost", "a.io"}
	for _, d := range validDomains {
		_, err := core.GenerateTLSCert(dir, "cert", d, 1)
		if err == nil {
			continue
		}
		if strings.Contains(err.Error(), "invalid domain") {
			t.Errorf("domain %q: got validation error, expected xray error: %v", d, err)
		}
	}
}

// TestGenerateTLSCert_PathInsideDir — базовая проверка, что итоговый путь
// лежит внутри переданной директории. Проверяем косвенно: после вызова
// (пусть и неуспешного) файл не должен появиться в родительской директории.
func TestGenerateTLSCert_PathInsideDir(t *testing.T) {
	core := &Core{binPath: "/bin/false"}

	parent := t.TempDir()
	dir := filepath.Join(parent, "certs")

	_, _ = core.GenerateTLSCert(dir, "cert", "example.com", 1)

	// В parent не должно появиться ничего, кроме certs/.
	entries, err := os.ReadDir(parent)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name() != "certs" {
			t.Errorf("unexpected entry in parent dir: %s", e.Name())
		}
	}
}
