package config

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"time"

	"nodebridge/internal/domain"
)

type Config struct {
	Server      ServerConfig  `json:"server"`
	Log         LogConfig     `json:"log"`
	Runtime     RuntimeConfig `json:"runtime"`
	Kernels     []Kernel      `json:"kernels"`
	Panels      []Panel       `json:"panels"`
	StaticNodes []domain.Node `json:"static_nodes"`
}

type ServerConfig struct {
	Listen string `json:"listen"`
	Token  string `json:"token"`
}

type LogConfig struct {
	Level string `json:"level"`
}

type RuntimeConfig struct {
	WorkDir        string `json:"work_dir"`
	SyncInterval   string `json:"sync_interval"`
	RequestTimeout string `json:"request_timeout"`
}

type Kernel struct {
	Name          string            `json:"name"`
	Type          string            `json:"type"`
	Executable    string            `json:"executable"`
	ConfigPath    string            `json:"config_path"`
	Enabled       bool              `json:"enabled"`
	VersionPolicy string            `json:"version_policy"`
	TargetVersion string            `json:"target_version"`
	Renderer      string            `json:"renderer"`
	Args          []string          `json:"args"`
	Env           map[string]string `json:"env"`
}

type Panel struct {
	Name       string            `json:"name"`
	Type       string            `json:"type"`
	APIVersion string            `json:"api_version"`
	APIHost    string            `json:"api_host"`
	APIKey     string            `json:"api_key"`
	NodeID     int               `json:"node_id"`
	NodeType   string            `json:"node_type"`
	ListenIP   string            `json:"listen_ip"`
	Enabled    bool              `json:"enabled"`
	Subscribe  SubscribeConfig   `json:"subscribe"`
	Cert       CertConfig        `json:"cert"`
	Headers    map[string]string `json:"headers"`
}

type SubscribeConfig struct {
	URL    string `json:"url"`
	Format string `json:"format"`
}

type CertConfig struct {
	Mode     string `json:"mode"`
	Domain   string `json:"domain"`
	CertFile string `json:"cert_file"`
	KeyFile  string `json:"key_file"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	cfg.setDefaults()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (c *Config) setDefaults() {
	if c.Server.Listen == "" {
		c.Server.Listen = "127.0.0.1:8088"
	}
	if c.Log.Level == "" {
		c.Log.Level = "info"
	}
	if c.Runtime.WorkDir == "" {
		c.Runtime.WorkDir = "var"
	}
	if c.Runtime.SyncInterval == "" {
		c.Runtime.SyncInterval = "60s"
	}
	if c.Runtime.RequestTimeout == "" {
		c.Runtime.RequestTimeout = "15s"
	}
	for i := range c.Panels {
		if c.Panels[i].Type == "" {
			c.Panels[i].Type = "v2board"
		}
		if c.Panels[i].APIVersion == "" {
			c.Panels[i].APIVersion = "v1"
		}
		if c.Panels[i].ListenIP == "" {
			c.Panels[i].ListenIP = "0.0.0.0"
		}
		if c.Panels[i].Cert.CertFile == "" {
			c.Panels[i].Cert.CertFile = "/etc/nodebridge/fullchain.cer"
		}
		if c.Panels[i].Cert.KeyFile == "" {
			c.Panels[i].Cert.KeyFile = "/etc/nodebridge/cert.key"
		}
	}
}

func (c *Config) Validate() error {
	if _, err := time.ParseDuration(c.Runtime.SyncInterval); err != nil {
		return fmt.Errorf("runtime.sync_interval: %w", err)
	}
	if _, err := time.ParseDuration(c.Runtime.RequestTimeout); err != nil {
		return fmt.Errorf("runtime.request_timeout: %w", err)
	}
	return nil
}

func (c *Config) SyncInterval() time.Duration {
	d, _ := time.ParseDuration(c.Runtime.SyncInterval)
	return d
}

func (c *Config) HTTPTimeout() time.Duration {
	d, _ := time.ParseDuration(c.Runtime.RequestTimeout)
	return d
}

func (l LogConfig) LevelValue() slog.Level {
	switch l.Level {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
