package kernel

import (
	"encoding/json"
	"testing"

	"nodebridge/internal/config"
	"nodebridge/internal/domain"
)

func TestRenderSingBoxAnyTLS(t *testing.T) {
	rendered, err := renderConfig(config.Kernel{Type: "sing-box", Renderer: "sing-box-1.12"}, []domain.Node{
		{
			Name:     "anytls-demo",
			Protocol: domain.ProtocolAnyTLS,
			ListenIP: "0.0.0.0",
			Port:     8443,
			Users: []domain.User{
				{ID: 1, Name: "1", UUID: "user-password", Password: "user-password"},
			},
			TLS: domain.TLSConfig{
				Enabled:    true,
				ServerName: "example.com",
				CertFile:   "/etc/nodebridge/fullchain.cer",
				KeyFile:    "/etc/nodebridge/cert.key",
			},
			PaddingScheme: []string{"stop=8"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	var cfg map[string]any
	if err := json.Unmarshal(rendered.Data, &cfg); err != nil {
		t.Fatal(err)
	}
	inbounds := cfg["inbounds"].([]any)
	first := inbounds[0].(map[string]any)
	if first["type"] != "anytls" {
		t.Fatalf("expected anytls inbound, got %v", first["type"])
	}
	if _, ok := first["tls"].(map[string]any); !ok {
		t.Fatal("expected tls block")
	}
}

func TestRenderHysteria2(t *testing.T) {
	rendered, err := renderConfig(config.Kernel{Type: "hysteria2"}, []domain.Node{
		{
			Name:     "hy2-demo",
			Protocol: domain.ProtocolHysteria2,
			Port:     8443,
			Users:    []domain.User{{ID: 1, Password: "secret"}},
			TLS: domain.TLSConfig{
				CertFile: "/etc/nodebridge/fullchain.cer",
				KeyFile:  "/etc/nodebridge/cert.key",
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if rendered.Format != "hysteria2" {
		t.Fatalf("unexpected format %s", rendered.Format)
	}
	if len(rendered.Data) == 0 {
		t.Fatal("empty hysteria2 config")
	}
}
