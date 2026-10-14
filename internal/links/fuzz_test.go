// SPDX-License-Identifier: AGPL-3.0-or-later

package links

import (
	"strings"
	"testing"
)

// FuzzGenerate проверяет, что Generate не паникует на любом
// вводе и возвращает либо ссылку, либо ошибку. Не проверяет
// семантику — только отсутствие паники и базовую структуру.
//
// Запуск: go test -fuzz=FuzzGenerate -fuzztime=30s ./internal/links/
func FuzzGenerate(f *testing.F) {
	// Корпус: валидные примеры для каждого протокола.
	f.Add("vless", "example.com", 443, `{"id":"11111111-1111-1111-1111-111111111111"}`, `{}`, `{}`)
	f.Add("vmess", "example.com", 443, `{"id":"11111111-1111-1111-1111-111111111111"}`, `{}`, `{}`)
	f.Add("trojan", "example.com", 443, `{"password":"secret"}`, `{}`, `{"security":"tls"}`)
	f.Add("shadowsocks", "example.com", 8388, `{"password":"secret"}`, `{"method":"aes-256-gcm"}`, `{}`)
	f.Add("hysteria", "example.com", 8443, `{"auth":"secret"}`, `{"version":2,"users":[{"auth":"secret"}]}`, `{"security":"tls"}`)

	f.Fuzz(func(t *testing.T, protocol, address string, port int, cred, settings, stream string) {
		// Ограничиваем размеры строк, чтобы fuzz не жрал память.
		if len(protocol) > 100 || len(address) > 1000 ||
			len(cred) > 10000 || len(settings) > 10000 || len(stream) > 10000 {
			t.Skip()
		}
		// port может быть любым int64/int — ограничим диапазон.
		if port < -1000 || port > 100000 {
			t.Skip()
		}

		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("panic on protocol=%q address=%q port=%d cred=%q settings=%q stream=%q: %v",
					protocol, address, port, cred, settings, stream, r)
			}
		}()

		got, err := Generate(Input{
			Protocol:       protocol,
			Address:        address,
			Port:           port,
			CredentialJSON: cred,
			SettingsJSON:   settings,
			StreamJSON:     stream,
			Remark:         "test",
		})

		// Если ссылка получена — она должна начинаться с ожидаемого
		// префикса. У некоторых протоколов префикс отличается от
		// имени в Input.Protocol (shadowsocks → ss://,
		// hysteria → hysteria2://).
		if err == nil {
			wantPrefix := schemeFor(protocol)
			if wantPrefix != "" && !strings.HasPrefix(got, wantPrefix) {
				t.Fatalf("got %q, expected prefix %q", got, wantPrefix)
			}
		}
	})
}

// schemeFor возвращает URL-префикс для протокола.
// Пустая строка — если протокол неизвестен, не проверяем.
func schemeFor(protocol string) string {
	switch protocol {
	case "vless":
		return "vless://"
	case "vmess":
		return "vmess://"
	case "trojan":
		return "trojan://"
	case "shadowsocks":
		return "ss://"
	case "hysteria":
		return "hysteria2://"
	case "socks":
		return "socks://"
	case "http":
		return "http://"
	default:
		return ""
	}
}
