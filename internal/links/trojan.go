// SPDX-License-Identifier: AGPL-3.0-or-later

package links

import (
	"fmt"
	"net/url"
)

// generateTrojan собирает trojan:// ссылку.
// Формат: trojan://<password>@<host>:<port>?<params>#<remark>
func generateTrojan(in Input) (string, error) {
	cred, err := parseJSON(in.CredentialJSON)
	if err != nil {
		return "", err
	}
	password := getString(cred, "password")
	if password == "" {
		return "", fmt.Errorf("trojan: credential has no password")
	}

	stream, err := parseJSON(in.StreamJSON)
	if err != nil {
		return "", err
	}

	q := url.Values{}
	q.Set("type", transportType(stream))
	q.Set("security", securityType(stream))

	if security := securityType(stream); security == "tls" || security == "reality" {
		tls, _ := stream["tlsSettings"].(map[string]any)
		reality, _ := stream["realitySettings"].(map[string]any)
		if tls != nil {
			if sni := getString(tls, "serverName"); sni != "" {
				q.Set("sni", sni)
			}
			if fp := getString(tls, "fingerprint"); fp != "" {
				q.Set("fp", fp)
			}
			if alpn, ok := tls["alpn"].([]any); ok {
				q.Set("alpn", joinAny(alpn))
			}
		}
		if reality != nil {
			if sni := getString(reality, "serverName"); sni != "" {
				q.Set("sni", sni)
			}
			if pbk := getString(reality, "publicKey"); pbk != "" {
				q.Set("pbk", pbk)
			}
		}
	}

	switch transportType(stream) {
	case "ws":
		ws, _ := stream["wsSettings"].(map[string]any)
		if ws != nil {
			if path := getString(ws, "path"); path != "" {
				q.Set("path", path)
			}
			if host := getString(ws, "host"); host != "" {
				q.Set("host", host)
			}
		}
	case "grpc":
		grpc, _ := stream["grpcSettings"].(map[string]any)
		if grpc != nil {
			if sn := getString(grpc, "serviceName"); sn != "" {
				q.Set("serviceName", sn)
			}
		}
	}

	remark := url.PathEscape(in.Remark)
	return fmt.Sprintf("trojan://%s@%s:%d?%s#%s",
		url.QueryEscape(password), hostForURL(in.Address), in.Port, q.Encode(), remark), nil
}
