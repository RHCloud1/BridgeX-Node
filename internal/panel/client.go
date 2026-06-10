package panel

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"nodebridge/internal/config"
	"nodebridge/internal/domain"
	"nodebridge/internal/subscription"
)

type Client interface {
	FetchSubscription(ctx context.Context, panel config.Panel) (domain.ImportResult, error)
	FetchServerNode(ctx context.Context, panel config.Panel) (domain.ImportResult, error)
}

type HTTPClient struct {
	client *http.Client
	parser subscription.Parser
}

func NewHTTPClient(timeout time.Duration) *HTTPClient {
	return &HTTPClient{
		client: &http.Client{Timeout: timeout},
		parser: subscription.NewParser(),
	}
}

func (c *HTTPClient) FetchSubscription(ctx context.Context, panelCfg config.Panel) (domain.ImportResult, error) {
	if panelCfg.Subscribe.URL == "" {
		return domain.ImportResult{Source: panelCfg.Name}, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, panelCfg.Subscribe.URL, nil)
	if err != nil {
		return domain.ImportResult{}, err
	}
	for key, value := range panelCfg.Headers {
		req.Header.Set(key, value)
	}
	if panelCfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+panelCfg.APIKey)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return domain.ImportResult{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return domain.ImportResult{}, fmt.Errorf("subscription status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return domain.ImportResult{}, err
	}
	return c.parser.Parse(panelCfg.Name, body)
}

func (c *HTTPClient) FetchServerNode(ctx context.Context, panelCfg config.Panel) (domain.ImportResult, error) {
	if panelCfg.APIHost == "" || panelCfg.APIKey == "" || panelCfg.NodeID == 0 || panelCfg.NodeType == "" {
		return domain.ImportResult{Source: panelCfg.Name}, nil
	}

	nodeConfig, err := c.fetchJSONMap(ctx, panelCfg, serverPath(panelCfg.APIVersion, "config"))
	if err != nil {
		return domain.ImportResult{}, err
	}
	users, err := c.fetchUsers(ctx, panelCfg)
	if err != nil {
		return domain.ImportResult{}, err
	}

	node := nodeFromPanel(panelCfg, nodeConfig, users)
	return domain.ImportResult{
		Source: panelCfg.Name,
		Count:  1,
		Nodes:  []domain.Node{node},
	}, nil
}

type panelUserList struct {
	Users []panelUser `json:"users"`
}

type panelUser struct {
	ID          int    `json:"id"`
	UUID        string `json:"uuid"`
	SpeedLimit  int    `json:"speed_limit"`
	DeviceLimit int    `json:"device_limit"`
}

func (c *HTTPClient) fetchUsers(ctx context.Context, panelCfg config.Panel) ([]domain.User, error) {
	var body panelUserList
	if err := c.fetchJSON(ctx, panelCfg, serverPath(panelCfg.APIVersion, "user"), &body); err != nil {
		return nil, err
	}
	users := make([]domain.User, 0, len(body.Users))
	for _, user := range body.Users {
		name := strconv.Itoa(user.ID)
		if name == "0" {
			name = user.UUID
		}
		users = append(users, domain.User{
			ID:          user.ID,
			Name:        name,
			UUID:        user.UUID,
			Password:    user.UUID,
			SpeedLimit:  user.SpeedLimit,
			DeviceLimit: user.DeviceLimit,
		})
	}
	return users, nil
}

func (c *HTTPClient) fetchJSONMap(ctx context.Context, panelCfg config.Panel, path string) (map[string]any, error) {
	var body map[string]any
	if err := c.fetchJSON(ctx, panelCfg, path, &body); err != nil {
		return nil, err
	}
	return body, nil
}

func (c *HTTPClient) fetchJSON(ctx context.Context, panelCfg config.Panel, path string, out any) error {
	endpoint, err := buildPanelURL(panelCfg, path)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	for key, value := range panelCfg.Headers {
		req.Header.Set(key, value)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotModified {
		return fmt.Errorf("panel returned 304 before local cache was populated")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return fmt.Errorf("panel %s status %d: %s", path, resp.StatusCode, strings.TrimSpace(string(data)))
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(out)
}

func buildPanelURL(panelCfg config.Panel, path string) (string, error) {
	base, err := url.Parse(panelCfg.APIHost)
	if err != nil {
		return "", err
	}
	base.Path = strings.TrimRight(base.Path, "/") + path
	q := base.Query()
	q.Set("node_id", strconv.Itoa(panelCfg.NodeID))
	q.Set("node_type", normalizeNodeType(panelCfg.NodeType))
	q.Set("token", panelCfg.APIKey)
	base.RawQuery = q.Encode()
	return base.String(), nil
}

func serverPath(version, name string) string {
	if strings.EqualFold(version, "v2") {
		return "/api/v2/server/" + name
	}
	return "/api/v1/server/UniProxy/" + name
}

func nodeFromPanel(panelCfg config.Panel, raw map[string]any, users []domain.User) domain.Node {
	protocol := domain.Protocol(normalizeNodeType(panelCfg.NodeType))
	serverName := firstString(raw, "server_name", "host")
	certConfig := mapField(raw, "cert_config")
	tlsSettings := mapField(raw, "tls_settings")
	if serverName == "" {
		serverName = stringFromMap(tlsSettings, "server_name", "serverName")
	}

	node := domain.Node{
		ID:          fmt.Sprintf("%s-%d-%s", panelCfg.Name, panelCfg.NodeID, protocol),
		Name:        fmt.Sprintf("%s-%d", panelCfg.Name, panelCfg.NodeID),
		PanelNodeID: panelCfg.NodeID,
		Protocol:    protocol,
		Address:     firstString(raw, "host", "server_name"),
		ListenIP:    panelCfg.ListenIP,
		Port:        intField(raw, "server_port", "port"),
		Method:      firstString(raw, "cipher", "method"),
		Network:     firstString(raw, "network"),
		SNI:         serverName,
		Host:        firstString(raw, "host"),
		Flow:        firstString(raw, "flow"),
		Users:       users,
		UpMbps:      intField(raw, "up_mbps"),
		DownMbps:    intField(raw, "down_mbps"),
		ObfsType:    firstString(raw, "obfs"),
		ObfsPassword: firstString(raw,
			"obfs-password",
			"obfs_password",
		),
		PaddingScheme: stringSlice(raw["padding_scheme"]),
		TLS: domain.TLSConfig{
			Enabled:    requiresTLS(protocol) || intField(raw, "tls") > 0,
			ServerName: serverName,
			CertFile:   firstNonEmpty(stringFromMap(certConfig, "cert_file", "CertFile"), panelCfg.Cert.CertFile),
			KeyFile:    firstNonEmpty(stringFromMap(certConfig, "key_file", "KeyFile"), panelCfg.Cert.KeyFile),
		},
		UpdatedAt: time.Now().UTC(),
		Source:    domain.SourceRef{Type: "panel", Name: panelCfg.Name},
		Meta: map[string]string{
			"api_version": panelCfg.APIVersion,
			"panel_type":  panelCfg.Type,
		},
	}
	if node.Method == "" && node.Protocol == domain.ProtocolSS {
		node.Method = "aes-128-gcm"
	}
	if node.Address == "" {
		node.Address = node.TLS.ServerName
	}
	return node
}

func normalizeNodeType(nodeType string) string {
	switch strings.ToLower(nodeType) {
	case "v2ray":
		return "vmess"
	case "sing":
		return "sing-box"
	default:
		return strings.ToLower(nodeType)
	}
}

func requiresTLS(protocol domain.Protocol) bool {
	switch protocol {
	case domain.ProtocolAnyTLS, domain.ProtocolHysteria, domain.ProtocolHysteria2, domain.ProtocolTrojan, domain.ProtocolTUIC:
		return true
	default:
		return false
	}
}

func intField(raw map[string]any, keys ...string) int {
	for _, key := range keys {
		switch value := raw[key].(type) {
		case float64:
			return int(value)
		case int:
			return value
		case string:
			parsed, _ := strconv.Atoi(value)
			return parsed
		}
	}
	return 0
}

func firstString(raw map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := raw[key].(string); ok && strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func mapField(raw map[string]any, key string) map[string]any {
	if value, ok := raw[key].(map[string]any); ok {
		return value
	}
	return nil
}

func stringFromMap(raw map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := raw[key].(string); ok && strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func stringSlice(value any) []string {
	switch list := value.(type) {
	case []string:
		return list
	case []any:
		out := make([]string, 0, len(list))
		for _, item := range list {
			if text, ok := item.(string); ok {
				out = append(out, text)
			}
		}
		return out
	default:
		return nil
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
