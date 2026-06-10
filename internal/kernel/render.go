package kernel

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"nodebridge/internal/config"
	"nodebridge/internal/domain"
)

type renderedConfig struct {
	Data   []byte
	Format string
	Ext    string
}

func renderConfig(kernelCfg config.Kernel, nodes []domain.Node) (renderedConfig, error) {
	renderer := kernelCfg.Renderer
	if renderer == "" {
		renderer = defaultRenderer(kernelCfg.Type)
	}
	switch {
	case strings.HasPrefix(renderer, "sing-box"), kernelCfg.Type == "sing", kernelCfg.Type == "sing-box":
		data, err := renderSingBox(nodes)
		return renderedConfig{Data: data, Format: "sing-box", Ext: ".json"}, err
	case strings.HasPrefix(renderer, "xray"), kernelCfg.Type == "xray":
		data, err := renderXray(nodes)
		return renderedConfig{Data: data, Format: "xray", Ext: ".json"}, err
	case strings.HasPrefix(renderer, "hysteria2"), kernelCfg.Type == "hysteria2":
		data, err := renderHysteria2(nodes)
		return renderedConfig{Data: data, Format: "hysteria2", Ext: ".yaml"}, err
	default:
		data, err := renderPlan(nodes)
		return renderedConfig{Data: data, Format: "plan", Ext: ".plan.json"}, err
	}
}

func defaultRenderer(kernelType string) string {
	switch kernelType {
	case "sing", "sing-box":
		return "sing-box-1.12"
	case "xray":
		return "xray-current"
	case "hysteria2":
		return "hysteria2-current"
	default:
		return "plan"
	}
}

func defaultConfigPath(workDir, name string, rendered renderedConfig) string {
	ext := rendered.Ext
	if ext == "" {
		ext = ".json"
	}
	return filepath.Join(workDir, name+ext)
}

func renderPlan(nodes []domain.Node) ([]byte, error) {
	plan := RenderedPlan{
		GeneratedAt: time.Now().UTC(),
		Nodes:       nodes,
		Note:        "Normalized NodeBridge plan. Configure renderer to emit native core config.",
	}
	return json.MarshalIndent(plan, "", "  ")
}

func renderSingBox(nodes []domain.Node) ([]byte, error) {
	inbounds := make([]map[string]any, 0, len(nodes))
	for _, node := range nodes {
		inbound, err := singInbound(node)
		if err != nil {
			return nil, err
		}
		if inbound != nil {
			inbounds = append(inbounds, inbound)
		}
	}
	cfg := map[string]any{
		"log": map[string]any{
			"level":     "info",
			"timestamp": true,
		},
		"inbounds": inbounds,
		"outbounds": []map[string]any{
			{"type": "direct", "tag": "direct"},
			{"type": "block", "tag": "block"},
		},
		"route": map[string]any{
			"final": "direct",
		},
	}
	return json.MarshalIndent(cfg, "", "  ")
}

func singInbound(node domain.Node) (map[string]any, error) {
	listen := node.ListenIP
	if listen == "" {
		listen = "0.0.0.0"
	}
	base := map[string]any{
		"type":        string(node.Protocol),
		"tag":         node.Name,
		"listen":      listen,
		"listen_port": node.Port,
	}
	if node.Port == 0 {
		return nil, fmt.Errorf("node %s has empty listen port", node.Name)
	}

	switch node.Protocol {
	case domain.ProtocolVMess:
		base["users"] = singUUIDUsers(node)
	case domain.ProtocolVLess:
		base["users"] = singVLESSUsers(node)
		putTLS(base, node)
		putTransport(base, node)
	case domain.ProtocolTrojan:
		base["users"] = singPasswordUsers(node)
		putTLS(base, node)
		putTransport(base, node)
	case domain.ProtocolSS:
		base["method"] = firstNonEmpty(node.Method, "aes-128-gcm")
		if node.Password != "" {
			base["password"] = node.Password
		}
		if len(node.Users) > 0 {
			base["users"] = singPasswordUsers(node)
		}
	case domain.ProtocolHysteria2:
		if node.UpMbps > 0 {
			base["up_mbps"] = node.UpMbps
		}
		if node.DownMbps > 0 {
			base["down_mbps"] = node.DownMbps
		}
		if node.ObfsType != "" {
			base["obfs"] = map[string]any{
				"type":     node.ObfsType,
				"password": node.ObfsPassword,
			}
		}
		base["users"] = singPasswordUsers(node)
		putTLS(base, node)
	case domain.ProtocolAnyTLS:
		base["users"] = singPasswordUsers(node)
		if len(node.PaddingScheme) > 0 {
			base["padding_scheme"] = node.PaddingScheme
		}
		putTLS(base, node)
	default:
		return nil, fmt.Errorf("sing-box renderer does not support %s yet", node.Protocol)
	}
	return base, nil
}

func renderXray(nodes []domain.Node) ([]byte, error) {
	inbounds := make([]map[string]any, 0, len(nodes))
	for _, node := range nodes {
		inbound, err := xrayInbound(node)
		if err != nil {
			return nil, err
		}
		if inbound != nil {
			inbounds = append(inbounds, inbound)
		}
	}
	cfg := map[string]any{
		"log": map[string]any{
			"loglevel": "warning",
		},
		"inbounds": inbounds,
		"outbounds": []map[string]any{
			{"protocol": "freedom", "tag": "direct"},
			{"protocol": "blackhole", "tag": "block"},
		},
	}
	return json.MarshalIndent(cfg, "", "  ")
}

func xrayInbound(node domain.Node) (map[string]any, error) {
	if node.Protocol == domain.ProtocolAnyTLS || node.Protocol == domain.ProtocolHysteria2 || node.Protocol == domain.ProtocolHysteria || node.Protocol == domain.ProtocolTUIC {
		return nil, nil
	}
	listen := node.ListenIP
	if listen == "" {
		listen = "0.0.0.0"
	}
	inbound := map[string]any{
		"tag":      node.Name,
		"listen":   listen,
		"port":     node.Port,
		"protocol": xrayProtocol(node.Protocol),
	}
	settings := map[string]any{}
	switch node.Protocol {
	case domain.ProtocolVMess:
		settings["clients"] = xrayIDUsers(node, false)
	case domain.ProtocolVLess:
		settings["clients"] = xrayIDUsers(node, true)
		settings["decryption"] = "none"
	case domain.ProtocolTrojan:
		settings["clients"] = xrayPasswordUsers(node)
	case domain.ProtocolSS:
		settings["method"] = firstNonEmpty(node.Method, "aes-128-gcm")
		settings["password"] = firstNonEmpty(node.Password, firstUserPassword(node))
		settings["network"] = "tcp,udp"
	}
	inbound["settings"] = settings
	stream := map[string]any{"network": firstNonEmpty(node.Network, "tcp")}
	if node.TLS.Enabled {
		stream["security"] = "tls"
		stream["tlsSettings"] = map[string]any{
			"serverName":   node.TLS.ServerName,
			"certificates": []map[string]any{{"certificateFile": node.TLS.CertFile, "keyFile": node.TLS.KeyFile}},
		}
	}
	inbound["streamSettings"] = stream
	return inbound, nil
}

func renderHysteria2(nodes []domain.Node) ([]byte, error) {
	for _, node := range nodes {
		if node.Protocol != domain.ProtocolHysteria2 {
			continue
		}
		var buf bytes.Buffer
		listen := node.ListenIP
		if listen == "" || listen == "0.0.0.0" || listen == "::" {
			listen = ":" + strconv.Itoa(node.Port)
		} else {
			listen = listen + ":" + strconv.Itoa(node.Port)
		}
		fmt.Fprintf(&buf, "listen: %q\n", listen)
		fmt.Fprintf(&buf, "tls:\n  cert: %q\n  key: %q\n", node.TLS.CertFile, node.TLS.KeyFile)
		if node.UpMbps > 0 || node.DownMbps > 0 {
			fmt.Fprintf(&buf, "bandwidth:\n  up: %d mbps\n  down: %d mbps\n", node.UpMbps, node.DownMbps)
		}
		if node.ObfsPassword != "" {
			fmt.Fprintf(&buf, "obfs:\n  type: %q\n  salamander:\n    password: %q\n", firstNonEmpty(node.ObfsType, "salamander"), node.ObfsPassword)
		}
		if len(node.Users) == 1 {
			fmt.Fprintf(&buf, "auth:\n  type: password\n  password: %q\n", firstUserPassword(node))
		} else {
			fmt.Fprintf(&buf, "auth:\n  type: userpass\n  userpass:\n")
			for _, user := range node.Users {
				fmt.Fprintf(&buf, "    %q: %q\n", firstNonEmpty(user.Name, strconv.Itoa(user.ID)), firstNonEmpty(user.Password, user.UUID))
			}
		}
		return buf.Bytes(), nil
	}
	return nil, fmt.Errorf("hysteria2 renderer requires at least one hysteria2 node")
}

func singUUIDUsers(node domain.Node) []map[string]any {
	users := make([]map[string]any, 0, max(1, len(node.Users)))
	if len(node.Users) == 0 && node.UserID != "" {
		return []map[string]any{{"name": node.Name, "uuid": node.UserID}}
	}
	for _, user := range node.Users {
		users = append(users, map[string]any{
			"name": firstNonEmpty(user.Name, strconv.Itoa(user.ID)),
			"uuid": firstNonEmpty(user.UUID, user.Password),
		})
	}
	return users
}

func singVLESSUsers(node domain.Node) []map[string]any {
	users := singUUIDUsers(node)
	if node.Flow == "" {
		return users
	}
	for _, user := range users {
		user["flow"] = node.Flow
	}
	return users
}

func singPasswordUsers(node domain.Node) []map[string]any {
	users := make([]map[string]any, 0, max(1, len(node.Users)))
	if len(node.Users) == 0 && node.Password != "" {
		return []map[string]any{{"name": node.Name, "password": node.Password}}
	}
	for _, user := range node.Users {
		users = append(users, map[string]any{
			"name":     firstNonEmpty(user.Name, strconv.Itoa(user.ID)),
			"password": firstNonEmpty(user.Password, user.UUID),
		})
	}
	return users
}

func xrayIDUsers(node domain.Node, includeFlow bool) []map[string]any {
	users := make([]map[string]any, 0, max(1, len(node.Users)))
	if len(node.Users) == 0 && node.UserID != "" {
		user := map[string]any{"id": node.UserID, "email": node.Name}
		if includeFlow && node.Flow != "" {
			user["flow"] = node.Flow
		}
		return []map[string]any{user}
	}
	for _, u := range node.Users {
		user := map[string]any{
			"id":    firstNonEmpty(u.UUID, u.Password),
			"email": firstNonEmpty(u.Name, strconv.Itoa(u.ID)),
		}
		if includeFlow && node.Flow != "" {
			user["flow"] = node.Flow
		}
		users = append(users, user)
	}
	return users
}

func xrayPasswordUsers(node domain.Node) []map[string]any {
	users := make([]map[string]any, 0, max(1, len(node.Users)))
	if len(node.Users) == 0 && node.Password != "" {
		return []map[string]any{{"password": node.Password, "email": node.Name}}
	}
	for _, u := range node.Users {
		users = append(users, map[string]any{
			"password": firstNonEmpty(u.Password, u.UUID),
			"email":    firstNonEmpty(u.Name, strconv.Itoa(u.ID)),
		})
	}
	return users
}

func putTLS(base map[string]any, node domain.Node) {
	if !node.TLS.Enabled {
		return
	}
	tls := map[string]any{"enabled": true}
	if node.TLS.ServerName != "" {
		tls["server_name"] = node.TLS.ServerName
	}
	if node.TLS.CertFile != "" {
		tls["certificate_path"] = node.TLS.CertFile
	}
	if node.TLS.KeyFile != "" {
		tls["key_path"] = node.TLS.KeyFile
	}
	base["tls"] = tls
}

func putTransport(base map[string]any, node domain.Node) {
	if node.Network == "" || node.Network == "tcp" {
		return
	}
	transport := map[string]any{"type": node.Network}
	if node.Path != "" {
		transport["path"] = node.Path
	}
	if node.Host != "" {
		transport["headers"] = map[string]any{"Host": node.Host}
	}
	base["transport"] = transport
}

func xrayProtocol(protocol domain.Protocol) string {
	if protocol == domain.ProtocolSS {
		return "shadowsocks"
	}
	return string(protocol)
}

func firstUserPassword(node domain.Node) string {
	for _, user := range node.Users {
		if user.Password != "" {
			return user.Password
		}
		if user.UUID != "" {
			return user.UUID
		}
	}
	return node.Password
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
