// SPDX-License-Identifier: AGPL-3.0-or-later

package links

import (
	"encoding/base64"
	"fmt"
)

// generateShadowsocks собирает ss:// ссылку.
// Формат: ss://base64(method:password)@host:port#remark
func generateShadowsocks(in Input) (string, error) {
	cred, err := parseJSON(in.CredentialJSON)
	if err != nil {
		return "", err
	}
	password := getString(cred, "password")
	if password == "" {
		return "", fmt.Errorf("shadowsocks: credential has no password")
	}

	settings, err := parseJSON(in.SettingsJSON)
	if err != nil {
		return "", err
	}
	method := getString(settings, "method")
	if method == "" {
		method = getString(cred, "method")
	}
	if method == "" {
		return "", fmt.Errorf("shadowsocks: method not set")
	}

	userinfo := base64.RawURLEncoding.EncodeToString([]byte(method + ":" + password))
	return fmt.Sprintf("ss://%s@%s:%d#%s",
		userinfo, hostForURL(in.Address), in.Port, in.Remark), nil
}
