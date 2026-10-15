// SPDX-License-Identifier: AGPL-3.0-or-later

package links

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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
		// serverNames и shortIds — массивы, как их хранит фронт.
		StreamJSON:       `{"network":"tcp","security":"reality","realitySettings":{"serverNames":["reality.example.com"],"shortIds":["abcd1234"],"fingerprint":"chrome","spiderX":"/path"}}`,
		RealityPublicKey: "PBK123",
		Remark:           "reality",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"security=reality",
		"pbk=PBK123",
		"sid=abcd1234",
		"sni=reality.example.com",
		"spx=%2Fpath",
		"fp=chrome",
	} {
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

func TestGenerateHysteria2(t *testing.T) {
	got, err := Generate(Input{
		Protocol:     "hysteria",
		Address:      "vpn.example.com",
		Port:         8443,
		SettingsJSON: `{"version":2,"users":[{"auth":"secret123","email":"user@gopanel"}]}`,
		StreamJSON:   `{"network":"hysteria","security":"tls","tlsSettings":{"certificates":[{"certificateFile":"/path/cert.crt","keyFile":"/path/cert.key"}],"alpn":["h3"]},"hysteriaSettings":{"version":2}}`,
		Remark:       "test",
	})
	if err != nil {
		t.Fatal(err)
	}

	// Проверяем префикс и основные параметры.
	if !strings.HasPrefix(got, "hysteria2://secret123@vpn.example.com:8443/") {
		t.Errorf("got %q, want prefix hysteria2://secret123@vpn.example.com:8443/", got)
	}
	// insecure/pinSHA256 появляются только если сертификат реально
	// существует и самоподписан. В тесте путь /path/cert.crt
	// не существует — certInfo вернёт (false, ""), флаги не
	// добавятся. Проверяем только безусловные параметры.
	for _, want := range []string{"alpn=h3", "#test"} {
		if !strings.Contains(got, want) {
			t.Errorf("got %q, missing %q", got, want)
		}
	}
}

func TestGenerateHysteria2IPv6(t *testing.T) {
	got, err := Generate(Input{
		Protocol:     "hysteria",
		Address:      "2001:db8::1",
		Port:         443,
		SettingsJSON: `{"version":2,"users":[{"auth":"pass","email":"u@gopanel"}]}`,
		StreamJSON:   `{"network":"hysteria","security":"tls","tlsSettings":{"certificates":[{"certificateFile":"/c","keyFile":"/k"}],"alpn":["h3"]}}`,
		Remark:       "ipv6",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "@[2001:db8::1]:443") {
		t.Errorf("got %q, want IPv6 in brackets", got)
	}
}

func TestGenerateHysteria2NoAuth(t *testing.T) {
	_, err := Generate(Input{
		Protocol:     "hysteria",
		Address:      "vpn.example.com",
		Port:         8443,
		SettingsJSON: `{"version":2,"users":[]}`,
		StreamJSON:   `{"network":"hysteria","security":"tls"}`,
	})
	if err == nil {
		t.Error("expected error when no auth found")
	}
}

func TestGenerateHysteria2SelfSigned(t *testing.T) {
	// Генерируем временный самоподписанный сертификат.
	dir := t.TempDir()
	certPath := filepath.Join(dir, "cert.pem")

	// Простой self-signed X.509 через crypto/x509.
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName:   "test",
			Organization: []string{"gopanel"},
		},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := Generate(Input{
		Protocol:     "hysteria",
		Address:      "vpn.example.com",
		Port:         8443,
		SettingsJSON: `{"version":2,"users":[{"auth":"secret123","email":"u@gopanel"}]}`,
		StreamJSON:   fmt.Sprintf(`{"network":"hysteria","security":"tls","tlsSettings":{"certificates":[{"certificateFile":%q,"keyFile":"/dev/null"}],"alpn":["h3"]}}`, certPath),
		Remark:       "test",
	})
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{"alpn=h3", "insecure=1", "pinSHA256=", "#test"} {
		if !strings.Contains(got, want) {
			t.Errorf("got %q, missing %q", got, want)
		}
	}
}

func TestGenerateVLESSRealityArrays(t *testing.T) {
	got, err := Generate(Input{
		Protocol:         "vless",
		Address:          "1.2.3.4",
		Port:             443,
		CredentialJSON:   `{"id":"11111111-1111-1111-1111-111111111111","flow":"xtls-rprx-vision"}`,
		StreamJSON:       `{"network":"tcp","security":"reality","realitySettings":{"serverNames":["reality.example.com"],"shortIds":["abcd1234"],"fingerprint":"chrome","spiderX":"/path"}}`,
		RealityPublicKey: "PBK123",
		Remark:           "reality",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"sni=reality.example.com", "sid=abcd1234", "spx=%2Fpath", "pbk=PBK123"} {
		if !strings.Contains(got, want) {
			t.Errorf("got %q, missing %q", got, want)
		}
	}
}
