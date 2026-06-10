package panel

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"nodebridge/internal/config"
	"nodebridge/internal/domain"
)

func TestFetchServerNodeFromUniProxy(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("node_id") != "7" || r.URL.Query().Get("token") != "secret" {
			http.Error(w, "bad query", http.StatusBadRequest)
			return
		}
		switch r.URL.Path {
		case "/api/v1/server/UniProxy/config":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"server_port":    8443,
				"server_name":    "example.com",
				"padding_scheme": []string{"stop=8"},
			})
		case "/api/v1/server/UniProxy/user":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"users": []map[string]any{{"id": 1, "uuid": "password-1"}},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	result, err := NewHTTPClient(5*time.Second).FetchServerNode(nilContext(t), config.Panel{
		Name:       "xboard",
		APIVersion: "v1",
		APIHost:    server.URL,
		APIKey:     "secret",
		NodeID:     7,
		NodeType:   "anytls",
		ListenIP:   "0.0.0.0",
		Cert:       config.CertConfig{CertFile: "/cert", KeyFile: "/key"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Count != 1 {
		t.Fatalf("expected 1 node, got %d", result.Count)
	}
	node := result.Nodes[0]
	if node.Protocol != domain.ProtocolAnyTLS || node.Port != 8443 || len(node.Users) != 1 {
		t.Fatalf("unexpected node: %+v", node)
	}
}

func nilContext(t *testing.T) context.Context {
	t.Helper()
	return context.Background()
}
