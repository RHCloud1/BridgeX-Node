package kernel

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"nodebridge/internal/config"
	"nodebridge/internal/domain"
)

type Manager struct {
	mu      sync.Mutex
	workDir string
	kernels []config.Kernel
	running map[string]*processState
	status  map[string]Status
}

type processState struct {
	cmd  *exec.Cmd
	done chan error
}

type Status struct {
	Name          string    `json:"name"`
	Type          string    `json:"type"`
	Enabled       bool      `json:"enabled"`
	Running       bool      `json:"running"`
	Config        string    `json:"config"`
	ConfigFormat  string    `json:"config_format,omitempty"`
	Renderer      string    `json:"renderer,omitempty"`
	TargetVersion string    `json:"target_version,omitempty"`
	VersionPolicy string    `json:"version_policy,omitempty"`
	UpdatedAt     time.Time `json:"updated_at"`
	Message       string    `json:"message,omitempty"`
}

type RenderedPlan struct {
	GeneratedAt time.Time     `json:"generated_at"`
	Nodes       []domain.Node `json:"nodes"`
	Note        string        `json:"note"`
}

func NewManager(kernels []config.Kernel, workDir string) *Manager {
	return &Manager{
		workDir: workDir,
		kernels: kernels,
		running: make(map[string]*processState),
		status:  make(map[string]Status),
	}
}

func (m *Manager) Apply(ctx context.Context, nodes []domain.Node) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := os.MkdirAll(m.workDir, 0o755); err != nil {
		return err
	}
	for _, kernelCfg := range m.kernels {
		name := kernelCfg.Name
		if name == "" {
			name = kernelCfg.Type
		}
		status := Status{
			Name:          name,
			Type:          kernelCfg.Type,
			Enabled:       kernelCfg.Enabled,
			Config:        kernelCfg.ConfigPath,
			Renderer:      kernelCfg.Renderer,
			TargetVersion: kernelCfg.TargetVersion,
			VersionPolicy: kernelCfg.VersionPolicy,
			UpdatedAt:     time.Now().UTC(),
		}
		if !kernelCfg.Enabled {
			status.Message = "disabled"
			m.status[name] = status
			continue
		}

		rendered, err := renderConfig(kernelCfg, nodes)
		if err != nil {
			status.Message = err.Error()
			m.status[name] = status
			return err
		}
		status.ConfigFormat = rendered.Format

		configPath := kernelCfg.ConfigPath
		if configPath == "" {
			configPath = defaultConfigPath(m.workDir, name, rendered)
		}
		if err := writeFile(configPath, rendered.Data); err != nil {
			status.Message = err.Error()
			m.status[name] = status
			return err
		}
		status.Config = configPath

		if kernelCfg.Executable == "" {
			status.Message = "executable not configured; rendered config only"
			m.status[name] = status
			continue
		}
		if err := validateConfig(ctx, kernelCfg, configPath); err != nil {
			status.Message = err.Error()
			m.status[name] = status
			return err
		}
		if err := m.restartLocked(ctx, name, kernelCfg, configPath); err != nil {
			status.Message = err.Error()
			m.status[name] = status
			return err
		}
		status.Running = true
		status.Message = "running"
		m.status[name] = status
	}
	return nil
}

func (m *Manager) Snapshot() []Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Status, 0, len(m.status))
	for _, status := range m.status {
		out = append(out, status)
	}
	return out
}

func (m *Manager) StopAll(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	var err error
	for name := range m.running {
		if stopErr := m.stopLocked(ctx, name); stopErr != nil {
			err = errors.Join(err, stopErr)
		}
	}
	return err
}

func (m *Manager) restartLocked(ctx context.Context, name string, kernelCfg config.Kernel, configPath string) error {
	if err := m.stopLocked(ctx, name); err != nil {
		return err
	}
	args := append([]string{}, kernelCfg.Args...)
	if len(args) == 0 {
		args = defaultArgs(kernelCfg.Type, configPath)
	}
	cmd := exec.Command(kernelCfg.Executable, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	for key, value := range kernelCfg.Env {
		cmd.Env = append(cmd.Env, key+"="+value)
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	state := &processState{cmd: cmd, done: make(chan error, 1)}
	m.running[name] = state
	go func() {
		err := cmd.Wait()
		state.done <- err
		close(state.done)
		if err != nil {
			slog.Warn("kernel exited", "name", name, "error", err)
		}
	}()
	return nil
}

func (m *Manager) stopLocked(ctx context.Context, name string) error {
	state, ok := m.running[name]
	if !ok || state.cmd.Process == nil {
		return nil
	}
	if err := state.cmd.Process.Signal(os.Interrupt); err != nil {
		_ = state.cmd.Process.Kill()
		delete(m.running, name)
		return err
	}
	select {
	case <-state.done:
	case <-ctx.Done():
		_ = state.cmd.Process.Kill()
		return ctx.Err()
	case <-time.After(5 * time.Second):
		_ = state.cmd.Process.Kill()
	}
	delete(m.running, name)
	return nil
}

func writeFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func validateConfig(ctx context.Context, kernelCfg config.Kernel, configPath string) error {
	args := validationArgs(kernelCfg.Type, configPath)
	if len(args) == 0 {
		return nil
	}
	validateCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(validateCtx, kernelCfg.Executable, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Stdout = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("validate %s config: %w: %s", kernelCfg.Type, err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

func validationArgs(kernelType, configPath string) []string {
	switch kernelType {
	case "sing", "sing-box":
		return []string{"check", "-c", configPath}
	case "xray":
		return []string{"run", "-test", "-config", configPath}
	default:
		return nil
	}
}

func defaultArgs(kernelType, configPath string) []string {
	switch kernelType {
	case "xray":
		return []string{"run", "-config", configPath}
	case "sing", "sing-box":
		return []string{"run", "-c", configPath}
	default:
		return []string{configPath}
	}
}

func (s Status) String() string {
	return fmt.Sprintf("%s/%s running=%t", s.Name, s.Type, s.Running)
}
