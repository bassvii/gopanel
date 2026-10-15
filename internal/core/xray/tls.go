// SPDX-License-Identifier: AGPL-3.0-or-later

package xray

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// TLSCertResult — результат генерации сертификата.
type TLSCertResult struct {
	CertPath string `json:"cert_path"`
	KeyPath  string `json:"key_path"`
	Domain   string `json:"domain"`
}

// CertNameRe — регулярка для имени файла сертификата.
// Экспортирована, чтобы api/xray.go не дублировал.
var CertNameRe = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

// CertDomainRe — регулярка для домена.
var CertDomainRe = regexp.MustCompile(`^(\*\.)?[A-Za-z0-9]([A-Za-z0-9-]{0,61}[A-Za-z0-9])?(\.[A-Za-z0-9]([A-Za-z0-9-]{0,61}[A-Za-z0-9])?)*$`)

func (c *Core) GenerateTLSCert(dir, name, domain string, expireHours int) (TLSCertResult, error) {
	if dir == "" {
		dir = "./certs"
	}
	if name == "" {
		name = "cert"
	}
	if domain == "" {
		domain = "localhost"
	}
	if expireHours <= 0 {
		expireHours = 8760 // 1 год
	}

	// Валидация входных данных.
	if !CertNameRe.MatchString(name) {
		return TLSCertResult{}, fmt.Errorf("invalid cert name: %q", name)
	}
	if domain != "localhost" && !CertDomainRe.MatchString(domain) {
		return TLSCertResult{}, fmt.Errorf("invalid domain: %q", domain)
	}

	if err := os.MkdirAll(dir, 0o700); err != nil {
		return TLSCertResult{}, fmt.Errorf("mkdir %s: %w", dir, err)
	}

	base := filepath.Join(dir, name)

	// Защита от traversal на случай, если валидация выше что-то пропустила.
	// cleanDir — абсолютный путь к каталогу с завершающим разделителем.
	// cleanBase — абсолютный путь к файлу.
	cleanDir, err := filepath.Abs(dir)
	if err != nil {
		return TLSCertResult{}, fmt.Errorf("resolve dir: %w", err)
	}
	cleanBase, err := filepath.Abs(base)
	if err != nil {
		return TLSCertResult{}, fmt.Errorf("resolve base: %w", err)
	}
	if !strings.HasPrefix(cleanBase, cleanDir+string(os.PathSeparator)) {
		return TLSCertResult{}, fmt.Errorf("cert path escapes directory: %q", cleanBase)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, c.binPath,
		"tls", "cert",
		"--domain="+domain,
		"--name=gopanel",
		"--file="+base,
		fmt.Sprintf("--expire=%dh", expireHours),
	)
	cmd.Stdin = nil
	out, err := cmd.CombinedOutput()
	if err != nil {
		return TLSCertResult{}, fmt.Errorf("xray tls cert failed: %w\n%s", err, out)
	}

	// Права выставляем только после успешной генерации.
	if err := os.Chmod(base+".key", 0o600); err != nil {
		return TLSCertResult{}, fmt.Errorf("chmod key: %w", err)
	}
	if err := os.Chmod(base+".crt", 0o644); err != nil {
		return TLSCertResult{}, fmt.Errorf("chmod cert: %w", err)
	}

	return TLSCertResult{
		CertPath: base + ".crt",
		KeyPath:  base + ".key",
		Domain:   domain,
	}, nil
}
