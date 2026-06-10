package subscription

import (
	"encoding/base64"
	"testing"
)

func TestParserSupportsBase64V2BoardStyleBody(t *testing.T) {
	vmess := `vmess://` + base64.StdEncoding.EncodeToString([]byte(`{"v":"2","ps":"demo","add":"example.com","port":"443","id":"uuid","net":"ws","host":"example.com","path":"/ws","tls":"tls"}`))
	body := base64.StdEncoding.EncodeToString([]byte(vmess + "\n" + "trojan://pass@example.org:443?sni=example.org#trojan-demo"))

	result, err := NewParser().Parse("test", []byte(body))
	if err != nil {
		t.Fatal(err)
	}
	if result.Count != 2 {
		t.Fatalf("expected 2 nodes, got %d", result.Count)
	}
	if result.Nodes[0].Name != "demo" {
		t.Fatalf("unexpected first node name: %s", result.Nodes[0].Name)
	}
}
