package service

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"nodebridge/internal/config"
	"nodebridge/internal/kernel"
	"nodebridge/internal/panel"
)

type API struct {
	cfg     *config.Config
	nodes   *Registry
	panel   panel.Client
	kernels *kernel.Manager
}

type importRequest struct {
	Name    string            `json:"name"`
	URL     string            `json:"url"`
	Format  string            `json:"format"`
	Headers map[string]string `json:"headers"`
}

func NewAPI(cfg *config.Config, nodes *Registry, panelClient panel.Client, kernels *kernel.Manager) *API {
	return &API{cfg: cfg, nodes: nodes, panel: panelClient, kernels: kernels}
}

func (a *API) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", a.health)
	mux.HandleFunc("GET /v1/nodes", a.auth(a.listNodes))
	mux.HandleFunc("POST /v1/import", a.auth(a.importSubscription))
	mux.HandleFunc("GET /v1/kernels", a.auth(a.listKernels))
	return mux
}

func (a *API) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *API) listNodes(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"nodes": a.nodes.List()})
}

func (a *API) importSubscription(w http.ResponseWriter, r *http.Request) {
	var req importRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if req.Name == "" {
		req.Name = "manual"
	}
	if req.URL == "" {
		writeError(w, http.StatusBadRequest, errors.New("url is required"))
		return
	}

	result, err := a.panel.FetchSubscription(r.Context(), config.Panel{
		Name:      req.Name,
		Type:      "subscription",
		Enabled:   true,
		Subscribe: config.SubscribeConfig{URL: req.URL, Format: req.Format},
		Headers:   req.Headers,
	})
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	a.nodes.ReplaceSource(req.Name, result.Nodes)
	if err := a.kernels.Apply(r.Context(), a.nodes.List()); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *API) listKernels(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"kernels": a.kernels.Snapshot()})
}

func (a *API) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if a.cfg.Server.Token == "" {
			next(w, r)
			return
		}
		header := r.Header.Get("Authorization")
		token := strings.TrimPrefix(header, "Bearer ")
		if token != a.cfg.Server.Token {
			writeError(w, http.StatusUnauthorized, errors.New("unauthorized"))
			return
		}
		next(w, r)
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}
