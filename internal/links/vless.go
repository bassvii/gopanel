// SPDX-License-Identifier: AGPL-3.0-or-later

package links

import (
	"fmt"
	"net/url"
	"strconv"
)

// generateVLESS собирает vless:// ссылку.
// Формат: vless://<uuid>@<host>:<port>?<params>#<remark>
func generateVLESS(in Input) (string, error) {
	cred, err := parseJSON(in.CredentialJSON)
	if err != nil {
		return "", err
	}
	uuid := getString(cred, "id")
	if uuid == "" {
		return "", fmt.Errorf("vless: credential has no id")
	}

	stream, err := parseJSON(in.StreamJSON)
	if err != nil {
		return "", err
	}

	q := url.Values{}
	q.Set("type", transportType(stream))
	q.Set("security", securityType(stream))

	// TLS/REALITY параметры
	if security := securityType(stream); security == "tls" || security == "reality" {
		tls, _ := stream["tlsSettings"].(map[string]any)
		reality, _ := stream["realitySettings"].(map[string]any)

		if security == "tls" && tls != nil {
			if sni := getString(tls, "serverName"); sni != "" {
				q.Set("sni", sni)
			}
			if alpn, ok := tls["alpn"].([]any); ok && len(alpn) > 0 {
				q.Set("alpn", joinAny(alpn))
			}
			if fp := getString(tls, "fingerprint"); fp != "" {
				q.Set("fp", fp)
			}
		}
		if security == "reality" && reality != nil {
			if sni := getString(reality, "serverName"); sni != "" {
				q.Set("sni", sni)
			}
			if pbk := getString(reality, "publicKey"); pbk != "" {
				q.Set("pbk", pbk)
			}
			if sid := getString(reality, "shortId"); sid != "" {
				q.Set("sid", sid)
			}
			if fp := getString(reality, "fingerprint"); fp != "" {
				q.Set("fp", fp)
			}
			q.Set("flow", getString(reality, "flow"))
		}
	}

	// Транспорт-специфичные параметры
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
	case "tcp":
		tcp, _ := stream["tcpSettings"].(map[string]any)
		if tcp != nil {
			if header, ok := tcp["header"].(map[string]any); ok {
				if t := getString(header, "type"); t != "" {
					q.Set("headerType", t)
				}
			}
		}
	}

	// flow из credential
	if flow := getString(cred, "flow"); flow != "" {
		q.Set("flow", flow)
	}

	remark := url.PathEscape(in.Remark)
	return fmt.Sprintf("vless://%s@%s:%d?%s#%s",
		uuid, hostForURL(in.Address), in.Port, q.Encode(), remark), nil
}

func transportType(stream map[string]any) string {
	if v := getString(stream, "network"); v != "" {
		return v
	}
	return "tcp"
}

func securityType(stream map[string]any) string {
	if v := getString(stream, "security"); v != "" {
		return v
	}
	return "none"
}

func hostForURL(host string) string {
	// IPv6 в URL должен быть в квадратных скобках.
	if len(host) > 0 && host[0] != '[' && isIPv6(host) {
		return "[" + host + "]"
	}
	return host
}

func isIPv6(host string) bool {
	// Грубая проверка: есть двоеточие и нет точки.
	for i := 0; i < len(host); i++ {
		if host[i] == ':' {
			return true
		}
		if host[i] == '.' {
			return false
		}
	}
	return false
}

func joinAny(a []any) string {
	out := ""
	for i, v := range a {
		if s, ok := v.(string); ok {
			if i > 0 {
				out += ","
			}
			out += s
		}
	}
	return out
}

var _ = strconv.Itoa // зарезервировано
