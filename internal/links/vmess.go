// SPDX-License-Identifier: AGPL-3.0-or-later

package links

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
)

// vmessConfig — JSON, который кодируется в base64 для vmess://.
type vmessConfig struct {
	V    string `json:"v"`
	PS   string `json:"ps"`
	Add  string `json:"add"`
	Port string `json:"port"`
	ID   string `json:"id"`
	Aid  string `json:"aid"`
	Scy  string `json:"scy"`
	Net  string `json:"net"`
	Type string `json:"type"`
	Host string `json:"host"`
	Path string `json:"path"`
	TLS  string `json:"tls"`
	SNI  string `json:"sni,omitempty"`
	ALPN string `json:"alpn,omitempty"`
	FP   string `json:"fp,omitempty"`
}

// generateVMess собирает vmess:// ссылку (base64 от JSON).
func generateVMess(in Input) (string, error) {
	cred, err := parseJSON(in.CredentialJSON)
	if err != nil {
		return "", err
	}
	uuid := getString(cred, "id")
	if uuid == "" {
		return "", fmt.Errorf("vmess: credential has no id")
	}

	stream, err := parseJSON(in.StreamJSON)
	if err != nil {
		return "", err
	}

	cfg := vmessConfig{
		V:    "2",
		PS:   in.Remark,
		Add:  in.Address,
		Port: fmt.Sprintf("%d", in.Port),
		ID:   uuid,
		Aid:  "0",
		Scy:  "auto",
		Net:  transportType(stream),
		Type: "none",
	}

	switch transportType(stream) {
	case "ws":
		ws, _ := stream["wsSettings"].(map[string]any)
		if ws != nil {
			cfg.Path = getString(ws, "path")
			cfg.Host = getString(ws, "host")
		}
	case "grpc":
		grpc, _ := stream["grpcSettings"].(map[string]any)
		if grpc != nil {
			cfg.Path = getString(grpc, "serviceName")
		}
	case "tcp":
		tcp, _ := stream["tcpSettings"].(map[string]any)
		if tcp != nil {
			if header, ok := tcp["header"].(map[string]any); ok {
				cfg.Type = getString(header, "type")
			}
		}
	}

	if security := securityType(stream); security == "tls" || security == "reality" {
		cfg.TLS = "tls"
		tls, _ := stream["tlsSettings"].(map[string]any)
		if tls != nil {
			cfg.SNI = getString(tls, "serverName")
			cfg.FP = getString(tls, "fingerprint")
			if alpn, ok := tls["alpn"].([]any); ok {
				cfg.ALPN = joinAny(alpn)
			}
		}
	}

	data, err := json.Marshal(cfg)
	if err != nil {
		return "", err
	}
	return "vmess://" + base64.StdEncoding.EncodeToString(data), nil
}
