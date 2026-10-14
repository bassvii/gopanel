// SPDX-License-Identifier: AGPL-3.0-or-later

package links

import (
	"fmt"
	"net/url"
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

	// Insecure, если сертификат самоподписанный.
	if certs, ok := tls["certificates"].([]any); ok && len(certs) > 0 {
		q.Set("insecure", "1")
	}

	remark := url.PathEscape(in.Remark)
	return fmt.Sprintf("hysteria2://%s@%s:%d/?%s#%s",
		url.PathEscape(auth), hostForURL(in.Address), in.Port, q.Encode(), remark), nil
}
