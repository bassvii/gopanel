// SPDX-License-Identifier: AGPL-3.0-or-later

package links

import (
	"strings"
	"testing"
)

func TestGenerateVLESSPlain(t *testing.T) {
	got, err := Generate(Input{
		Protocol:       "vless",
		Address:        "example.com",
		Port:           443,
		CredentialJSON: `{"id":"11111111-1111-1111-1111-111111111111"}`,
		StreamJSON:     `{}`,
		SettingsJSON:   `{"decryption":"none"}`,
		Remark:         "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	wantPrefix := "vless://11111111-1111-1111-1111-111111111111@example.com:443?"
	if !strings.HasPrefix(got, wantPrefix) {
		t.Errorf("got %q, want prefix %q", got, wantPrefix)
	}
	if !strings.HasSuffix(got, "#test") {
		t.Errorf("got %q, want suffix #test", got)
	}
}

func TestGenerateVLESSTLS(t *testing.T) {
	got, err := Generate(Input{
		Protocol:       "vless",
		Address:        "example.com",
		Port:           443,
		CredentialJSON: `{"id":"11111111-1111-1111-1111-111111111111"}`,
		StreamJSON:     `{"network":"ws","security":"tls","tlsSettings":{"serverName":"sni.example.com"},"wsSettings":{"path":"/ws"}}`,
		Remark:         "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"type=ws", "security=tls", "sni=sni.example.com", "path=%2Fws"} {
		if !strings.Contains(got, want) {
			t.Errorf("got %q, missing %q", got, want)
		}
	}
}

func TestGenerateVLESSTLSReality(t *testing.T) {
	got, err := Generate(Input{
		Protocol:       "vless",
		Address:        "1.2.3.4",
		Port:           443,
		CredentialJSON: `{"id":"11111111-1111-1111-1111-111111111111","flow":"xtls-rprx-vision"}`,
		StreamJSON:     `{"network":"tcp","security":"reality","realitySettings":{"serverName":"reality.example.com","publicKey":"PBK123","shortId":"abcd1234","fingerprint":"chrome"}}`,
		Remark:         "reality",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"security=reality", "pbk=PBK123", "sid=abcd1234", "fp=chrome", "sni=reality.example.com"} {
		if !strings.Contains(got, want) {
			t.Errorf("got %q, missing %q", got, want)
		}
	}
}

func TestGenerateVMess(t *testing.T) {
	got, err := Generate(Input{
		Protocol:       "vmess",
		Address:        "example.com",
		Port:           443,
		CredentialJSON: `{"id":"11111111-1111-1111-1111-111111111111"}`,
		StreamJSON:     `{}`,
		Remark:         "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(got, "vmess://") {
		t.Errorf("got %q, want vmess:// prefix", got)
	}
}

func TestGenerateTrojan(t *testing.T) {
	got, err := Generate(Input{
		Protocol:       "trojan",
		Address:        "example.com",
		Port:           443,
		CredentialJSON: `{"password":"secret123"}`,
		StreamJSON:     `{"security":"tls","tlsSettings":{"serverName":"sni.example.com"}}`,
		Remark:         "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"trojan://secret123@example.com:443", "security=tls", "sni=sni.example.com"} {
		if !strings.Contains(got, want) {
			t.Errorf("got %q, missing %q", got, want)
		}
	}
}

func TestGenerateShadowsocks(t *testing.T) {
	got, err := Generate(Input{
		Protocol:       "shadowsocks",
		Address:        "example.com",
		Port:           8388,
		CredentialJSON: `{"password":"secret123"}`,
		SettingsJSON:   `{"method":"aes-256-gcm"}`,
		Remark:         "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(got, "ss://") {
		t.Errorf("got %q, want ss:// prefix", got)
	}
	if !strings.HasSuffix(got, "#test") {
		t.Errorf("got %q, want suffix #test", got)
	}
}

func TestGenerateIPv6(t *testing.T) {
	got, err := Generate(Input{
		Protocol:       "vless",
		Address:        "2001:db8::1",
		Port:           443,
		CredentialJSON: `{"id":"11111111-1111-1111-1111-111111111111"}`,
		StreamJSON:     `{}`,
		Remark:         "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "@[2001:db8::1]:443") {
		t.Errorf("got %q, want IPv6 in brackets", got)
	}
}

func TestGenerateUnsupported(t *testing.T) {
	_, err := Generate(Input{
		Protocol:       "hysteria2",
		Address:        "example.com",
		Port:           443,
		CredentialJSON: `{}`,
	})
	if err == nil {
		t.Error("expected error for unsupported protocol")
	}
}
