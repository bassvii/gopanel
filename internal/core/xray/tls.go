// SPDX-License-Identifier: AGPL-3.0-or-later

package xray

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// TLSCertResult — результат генерации сертификата.
type TLSCertResult struct {
	CertPath string `json:"cert_path"`
	KeyPath  string `json:"key_path"`
	Domain   string `json:"domain"`
}

// GenerateTLSCert создаёт самоподписанный сертификат через `xray tls cert`.
// dir — каталог для файлов, name — базовое имя (без расширения).
// domain — CN сертификата. expireHours — срок в часах.
func (c *Core) GenerateTLSCert(ctx context.Context, dir, name, domain string, expireHours int) (TLSCertResult, error) {
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

	if err := os.MkdirAll(dir, 0o700); err != nil {
		return TLSCertResult{}, fmt.Errorf("mkdir %s: %w", dir, err)
	}

	base := filepath.Join(dir, name)

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
	_ = os.Chmod(base+".key", 0o600)
	_ = os.Chmod(base+".crt", 0o644)
	if err != nil {
		return TLSCertResult{}, fmt.Errorf("xray tls cert failed: %w\n%s", err, out)
	}

	return TLSCertResult{
		CertPath: base + ".crt",
		KeyPath:  base + ".key",
		Domain:   domain,
	}, nil
}
