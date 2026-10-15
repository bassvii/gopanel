// SPDX-License-Identifier: AGPL-3.0-or-later

package links

import (
	"bytes"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"net/url"
	"os"
)

// generateHysteria2 собирает hysteria2:// ссылку.
func generateHysteria2(in Input) (string, error) {
	cred, err := parseJSON(in.CredentialJSON)
	if err != nil {
		return "", err
	}
	// Пароль (auth) — из credential или settings.
	auth := getString(cred, "auth")
	if auth == "" {
		settings, _ := parseJSON(in.SettingsJSON)
		users, _ := settings["users"].([]any)
		if len(users) > 0 {
			first, _ := users[0].(map[string]any)
			auth = getString(first, "auth")
		}
	}
	if auth == "" {
		return "", fmt.Errorf("hysteria2: no auth found in credential or settings")
	}

	stream, err := parseJSON(in.StreamJSON)
	if err != nil {
		return "", err
	}
	tls, _ := stream["tlsSettings"].(map[string]any)

	q := url.Values{}
	q.Set("alpn", "h3")

	// SNI.
	sni := getString(tls, "serverName")
	if sni != "" {
		q.Set("sni", sni)
	}

	// Если сертификат самоподписанный — insecure=1 (иначе клиент
	// не может проверить цепочку) + pinSHA256 (клиент принимает
	// именно этот сертификат, а не любой). См. #3 нового аудита.
	//
	// Если сертификат валидный (Let's Encrypt и т.п.) — insecure
	// НЕ ставим, клиент проверит цепочку сам.
	if certs, ok := tls["certificates"].([]any); ok && len(certs) > 0 {
		first, _ := certs[0].(map[string]any)
		certPath := getString(first, "certificateFile")
		if self, pin := certInfo(certPath); self {
			q.Set("insecure", "1")
			if pin != "" {
				q.Set("pinSHA256", pin)
			}
		}
	}

	remark := url.PathEscape(in.Remark)
	return fmt.Sprintf("hysteria2://%s@%s:%d/?%s#%s",
		url.PathEscape(auth), hostForURL(in.Address), in.Port, q.Encode(), remark), nil

}

// certInfo читает X.509-сертификат с диска и возвращает:
//   - selfSigned: true, если subject == issuer (самоподписанный);
//   - pin: SHA-256 отпечаток сертификата в hex (для pinSHA256).
//
// Используется в generateHysteria2, чтобы не ставить insecure=1
// для валидных сертификатов. См. #3 нового аудита.
func certInfo(path string) (selfSigned bool, pin string) {
	if path == "" {
		return false, ""
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return false, ""
	}
	blk, _ := pem.Decode(raw)
	if blk == nil {
		return false, ""
	}
	cert, err := x509.ParseCertificate(blk.Bytes)
	if err != nil {
		return false, ""
	}

	sum := sha256.Sum256(cert.Raw)
	pin = hex.EncodeToString(sum[:])

	// CheckSignatureFrom требует KeyUsageCertSign (сертификат УЦ).
	// У самоподписанного leaf-сертификата его может не быть.
	// Поэтому проверяем только RawIssuer == RawSubject — этого
	// достаточно, чтобы отличить self-signed от CA-signed.
	selfSigned = bytes.Equal(cert.RawIssuer, cert.RawSubject)

	return selfSigned, pin
}
