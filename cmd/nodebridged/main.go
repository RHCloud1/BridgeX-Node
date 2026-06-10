package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"nodebridge/internal/config"
	"nodebridge/internal/kernel"
	"nodebridge/internal/panel"
	"nodebridge/internal/service"
)

var version = "dev"

func main() {
	configPath := flag.String("config", "configs/nodebridge.example.json", "path to nodebridge config")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println(version)
		return
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Error("load config", "error", err)
		os.Exit(1)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: cfg.Log.LevelValue(),
	}))
	slog.SetDefault(logger)

	registry := service.NewRegistry()
	panelClient := panel.NewHTTPClient(cfg.HTTPTimeout())
	kernelManager := kernel.NewManager(cfg.Kernels, cfg.Runtime.WorkDir)

	syncer := service.NewSyncer(cfg, panelClient, registry, kernelManager)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := syncer.Start(ctx); err != nil {
		slog.Error("start syncer", "error", err)
		os.Exit(1)
	}

	api := service.NewAPI(cfg, registry, panelClient, kernelManager)
	server := &http.Server{
		Addr:              cfg.Server.Listen,
		Handler:           api.Routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		slog.Info("nodebridge api listening", "addr", cfg.Server.Listen)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("api server stopped", "error", err)
			stop()
		}
	}()

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Warn("api shutdown", "error", err)
	}
	if err := kernelManager.StopAll(shutdownCtx); err != nil {
		slog.Warn("kernel shutdown", "error", err)
	}
	slog.Info("nodebridge stopped")
}
