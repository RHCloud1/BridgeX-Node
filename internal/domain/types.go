package domain

import "time"

type Protocol string

const (
	ProtocolVMess     Protocol = "vmess"
	ProtocolVLess     Protocol = "vless"
	ProtocolTrojan    Protocol = "trojan"
	ProtocolSS        Protocol = "shadowsocks"
	ProtocolHysteria  Protocol = "hysteria"
	ProtocolHysteria2 Protocol = "hysteria2"
	ProtocolTUIC      Protocol = "tuic"
	ProtocolAnyTLS    Protocol = "anytls"
	ProtocolUnknown   Protocol = "unknown"
)

type Node struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	PanelNodeID   int               `json:"panel_node_id,omitempty"`
	Protocol      Protocol          `json:"protocol"`
	Address       string            `json:"address"`
	ListenIP      string            `json:"listen_ip,omitempty"`
	Port          int               `json:"port"`
	UserID        string            `json:"user_id,omitempty"`
	Password      string            `json:"password,omitempty"`
	Method        string            `json:"method,omitempty"`
	Network       string            `json:"network,omitempty"`
	SNI           string            `json:"sni,omitempty"`
	Path          string            `json:"path,omitempty"`
	Host          string            `json:"host,omitempty"`
	Flow          string            `json:"flow,omitempty"`
	AllowInsec    bool              `json:"allow_insecure,omitempty"`
	Users         []User            `json:"users,omitempty"`
	TLS           TLSConfig         `json:"tls,omitempty"`
	UpMbps        int               `json:"up_mbps,omitempty"`
	DownMbps      int               `json:"down_mbps,omitempty"`
	ObfsType      string            `json:"obfs_type,omitempty"`
	ObfsPassword  string            `json:"obfs_password,omitempty"`
	PaddingScheme []string          `json:"padding_scheme,omitempty"`
	Tags          []string          `json:"tags,omitempty"`
	Raw           string            `json:"raw,omitempty"`
	Meta          map[string]string `json:"meta,omitempty"`
	Source        SourceRef         `json:"source"`
	UpdatedAt     time.Time         `json:"updated_at"`
}

type User struct {
	ID          int    `json:"id,omitempty"`
	Name        string `json:"name,omitempty"`
	UUID        string `json:"uuid,omitempty"`
	Password    string `json:"password,omitempty"`
	SpeedLimit  int    `json:"speed_limit,omitempty"`
	DeviceLimit int    `json:"device_limit,omitempty"`
}

type TLSConfig struct {
	Enabled    bool   `json:"enabled,omitempty"`
	ServerName string `json:"server_name,omitempty"`
	CertFile   string `json:"cert_file,omitempty"`
	KeyFile    string `json:"key_file,omitempty"`
}

type SourceRef struct {
	Type string `json:"type"`
	Name string `json:"name"`
}

type ImportResult struct {
	Source string `json:"source"`
	Count  int    `json:"count"`
	Nodes  []Node `json:"nodes"`
}
