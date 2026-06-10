package subscription

import (
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"nodebridge/internal/domain"
)

type Parser struct{}

func NewParser() Parser {
	return Parser{}
}

func (Parser) Parse(sourceName string, body []byte) (domain.ImportResult, error) {
	text := strings.TrimSpace(string(body))
	if text == "" {
		return domain.ImportResult{Source: sourceName}, nil
	}

	lines := splitLines(text)
	if len(lines) == 1 && !looksLikeLink(lines[0]) {
		decoded, err := decodeBase64Text(lines[0])
		if err == nil {
			lines = splitLines(decoded)
		}
	}

	nodes := make([]domain.Node, 0, len(lines))
	for _, line := range lines {
		node, err := parseLink(sourceName, line)
		if err != nil {
			continue
		}
		nodes = append(nodes, node)
	}

	return domain.ImportResult{
		Source: sourceName,
		Count:  len(nodes),
		Nodes:  nodes,
	}, nil
}

func splitLines(text string) []string {
	raw := strings.FieldsFunc(text, func(r rune) bool {
		return r == '\n' || r == '\r'
	})
	lines := make([]string, 0, len(raw))
	for _, item := range raw {
		item = strings.TrimSpace(item)
		if item != "" {
			lines = append(lines, item)
		}
	}
	return lines
}

func looksLikeLink(line string) bool {
	return strings.Contains(line, "://")
}

func decodeBase64Text(value string) (string, error) {
	normalized := strings.TrimSpace(value)
	if mod := len(normalized) % 4; mod != 0 {
		normalized += strings.Repeat("=", 4-mod)
	}
	decoders := []*base64.Encoding{
		base64.StdEncoding,
		base64.URLEncoding,
		base64.RawStdEncoding,
		base64.RawURLEncoding,
	}
	var lastErr error
	for _, decoder := range decoders {
		data, err := decoder.DecodeString(normalized)
		if err == nil {
			return string(data), nil
		}
		lastErr = err
	}
	return "", lastErr
}

func parseLink(sourceName, raw string) (domain.Node, error) {
	switch {
	case strings.HasPrefix(raw, "vmess://"):
		return parseVMess(sourceName, raw)
	case strings.HasPrefix(raw, "vless://"):
		return parseURLNode(sourceName, raw, domain.ProtocolVLess)
	case strings.HasPrefix(raw, "trojan://"):
		return parseURLNode(sourceName, raw, domain.ProtocolTrojan)
	case strings.HasPrefix(raw, "ss://"):
		return parseShadowsocks(sourceName, raw)
	case strings.HasPrefix(raw, "hysteria://"):
		return parseURLNode(sourceName, raw, domain.ProtocolHysteria)
	case strings.HasPrefix(raw, "hysteria2://"), strings.HasPrefix(raw, "hy2://"):
		return parseURLNode(sourceName, raw, domain.ProtocolHysteria2)
	case strings.HasPrefix(raw, "tuic://"):
		return parseURLNode(sourceName, raw, domain.ProtocolTUIC)
	case strings.HasPrefix(raw, "anytls://"):
		return parseURLNode(sourceName, raw, domain.ProtocolAnyTLS)
	default:
		return domain.Node{}, fmt.Errorf("unsupported subscription link")
	}
}

type vmessPayload struct {
	Version string `json:"v"`
	Name    string `json:"ps"`
	Address string `json:"add"`
	Port    any    `json:"port"`
	UserID  string `json:"id"`
	Network string `json:"net"`
	Type    string `json:"type"`
	Host    string `json:"host"`
	Path    string `json:"path"`
	TLS     string `json:"tls"`
	SNI     string `json:"sni"`
}

func parseVMess(sourceName, raw string) (domain.Node, error) {
	payload := strings.TrimPrefix(raw, "vmess://")
	decoded, err := decodeBase64Text(payload)
	if err != nil {
		return domain.Node{}, err
	}
	var vm vmessPayload
	if err := json.Unmarshal([]byte(decoded), &vm); err != nil {
		return domain.Node{}, err
	}
	port, _ := intFromAny(vm.Port)
	node := domain.Node{
		ID:        stableID(raw),
		Name:      fallbackName(vm.Name, vm.Address),
		Protocol:  domain.ProtocolVMess,
		Address:   vm.Address,
		Port:      port,
		UserID:    vm.UserID,
		Network:   vm.Network,
		Host:      vm.Host,
		Path:      vm.Path,
		SNI:       firstNonEmpty(vm.SNI, vm.Host),
		Raw:       raw,
		UpdatedAt: time.Now().UTC(),
		Source:    domain.SourceRef{Type: "subscription", Name: sourceName},
		Meta: map[string]string{
			"tls":  vm.TLS,
			"type": vm.Type,
		},
	}
	return node, nil
}

func parseURLNode(sourceName, raw string, protocol domain.Protocol) (domain.Node, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return domain.Node{}, err
	}
	port, _ := strconv.Atoi(u.Port())
	query := u.Query()
	userName := u.User.Username()
	pass := password(u)
	if protocol == domain.ProtocolTrojan && pass == "" {
		pass = userName
	}
	node := domain.Node{
		ID:         stableID(raw),
		Name:       fallbackName(fragmentName(u), u.Hostname()),
		Protocol:   protocol,
		Address:    u.Hostname(),
		Port:       port,
		UserID:     userName,
		Password:   pass,
		Network:    firstNonEmpty(query.Get("type"), query.Get("network")),
		SNI:        firstNonEmpty(query.Get("sni"), query.Get("peer"), query.Get("host")),
		Path:       query.Get("path"),
		Host:       query.Get("host"),
		Flow:       query.Get("flow"),
		AllowInsec: truthy(query.Get("allowInsecure")) || truthy(query.Get("insecure")),
		Raw:        raw,
		UpdatedAt:  time.Now().UTC(),
		Source:     domain.SourceRef{Type: "subscription", Name: sourceName},
		Meta:       queryToMap(query),
	}
	return node, nil
}

func parseShadowsocks(sourceName, raw string) (domain.Node, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return domain.Node{}, err
	}

	name := fallbackName(fragmentName(u), u.Hostname())
	method, passwordValue := "", ""
	address, port := u.Hostname(), 0
	if parsedPort, err := strconv.Atoi(u.Port()); err == nil {
		port = parsedPort
	}

	userInfo := strings.TrimPrefix(raw, "ss://")
	if cut, _, ok := strings.Cut(userInfo, "#"); ok {
		userInfo = cut
	}
	if cut, _, ok := strings.Cut(userInfo, "?"); ok {
		userInfo = cut
	}

	if u.User != nil && u.Hostname() != "" {
		userName := u.User.Username()
		pass := password(u)
		if pass == "" {
			if decoded, err := decodeBase64Text(userName); err == nil {
				method, passwordValue, _ = strings.Cut(decoded, ":")
			} else {
				method = userName
			}
		} else {
			method = userName
			passwordValue = pass
		}
	} else {
		decoded, err := decodeBase64Text(userInfo)
		if err == nil {
			if auth, host, ok := strings.Cut(decoded, "@"); ok {
				method, passwordValue, _ = strings.Cut(auth, ":")
				address, port = splitHostPort(host)
			}
		}
	}

	return domain.Node{
		ID:        stableID(raw),
		Name:      name,
		Protocol:  domain.ProtocolSS,
		Address:   address,
		Port:      port,
		Method:    method,
		Password:  passwordValue,
		Raw:       raw,
		UpdatedAt: time.Now().UTC(),
		Source:    domain.SourceRef{Type: "subscription", Name: sourceName},
		Meta:      queryToMap(u.Query()),
	}, nil
}

func stableID(raw string) string {
	sum := sha1.Sum([]byte(raw))
	return hex.EncodeToString(sum[:])[:16]
}

func fragmentName(u *url.URL) string {
	if u.Fragment == "" {
		return ""
	}
	name, err := url.QueryUnescape(u.Fragment)
	if err != nil {
		return u.Fragment
	}
	return name
}

func password(u *url.URL) string {
	if u.User == nil {
		return ""
	}
	value, _ := u.User.Password()
	return value
}

func queryToMap(values url.Values) map[string]string {
	out := make(map[string]string, len(values))
	for key, value := range values {
		if len(value) > 0 {
			out[key] = value[0]
		}
	}
	return out
}

func intFromAny(value any) (int, error) {
	switch v := value.(type) {
	case float64:
		return int(v), nil
	case string:
		return strconv.Atoi(v)
	default:
		return 0, fmt.Errorf("unsupported int type %T", value)
	}
}

func splitHostPort(value string) (string, int) {
	host, portText, err := net.SplitHostPort(value)
	if err != nil {
		return value, 0
	}
	port, _ := strconv.Atoi(portText)
	return host, port
}

func fallbackName(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return "unnamed-node"
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func truthy(value string) bool {
	switch strings.ToLower(value) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
