package service

import (
	"context"
	"log/slog"
	"time"

	"nodebridge/internal/config"
	"nodebridge/internal/domain"
	"nodebridge/internal/kernel"
	"nodebridge/internal/panel"
)

type Syncer struct {
	cfg     *config.Config
	panel   panel.Client
	nodes   *Registry
	kernels *kernel.Manager
}

func NewSyncer(cfg *config.Config, panelClient panel.Client, nodes *Registry, kernels *kernel.Manager) *Syncer {
	return &Syncer{cfg: cfg, panel: panelClient, nodes: nodes, kernels: kernels}
}

func (s *Syncer) Start(ctx context.Context) error {
	if err := s.SyncOnce(ctx); err != nil {
		slog.Warn("initial sync failed", "error", err)
	}
	go s.loop(ctx)
	return nil
}

func (s *Syncer) loop(ctx context.Context) {
	ticker := time.NewTicker(s.cfg.SyncInterval())
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.SyncOnce(ctx); err != nil {
				slog.Warn("periodic sync failed", "error", err)
			}
		}
	}
}

func (s *Syncer) SyncOnce(ctx context.Context) error {
	if len(s.cfg.StaticNodes) > 0 {
		nodes := make([]domain.Node, len(s.cfg.StaticNodes))
		copy(nodes, s.cfg.StaticNodes)
		for i := range nodes {
			if nodes[i].Source.Name == "" {
				nodes[i].Source = domain.SourceRef{Type: "static", Name: "config"}
			}
			if nodes[i].ListenIP == "" {
				nodes[i].ListenIP = "0.0.0.0"
			}
		}
		s.nodes.ReplaceSource("config", nodes)
	}
	for _, panelCfg := range s.cfg.Panels {
		if !panelCfg.Enabled {
			continue
		}
		result, err := s.fetchPanel(ctx, panelCfg)
		if err != nil {
			slog.Warn("fetch panel failed", "panel", panelCfg.Name, "error", err)
			continue
		}
		s.nodes.ReplaceSource(panelCfg.Name, result.Nodes)
		slog.Info("panel synced", "panel", panelCfg.Name, "nodes", result.Count)
	}
	return s.kernels.Apply(ctx, s.nodes.List())
}

func (s *Syncer) fetchPanel(ctx context.Context, panelCfg config.Panel) (domain.ImportResult, error) {
	if panelCfg.Subscribe.URL != "" {
		return s.panel.FetchSubscription(ctx, panelCfg)
	}
	return s.panel.FetchServerNode(ctx, panelCfg)
}
