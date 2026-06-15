# NodeBridge

NodeBridge is a server-side proxy control-plane scaffold for importing panel subscriptions, normalizing nodes, and orchestrating external proxy kernels such as sing-box or Xray.

This repository is intentionally split into small modules so it can grow into an Xboard/V2bX-style node service without binding the whole project to one panel or one kernel.

## Current Scope

- Import common subscription bodies, including classic V2Board/Xboard base64 line lists.
- Parse common share links: VMess, VLESS, Trojan, Shadowsocks, Hysteria, Hysteria2, TUIC, and AnyTLS.
- Expose a small HTTP API for health, manual import, node listing, and kernel status.
- Periodically sync configured panel subscriptions.
- Render a normalized kernel plan file for each configured core.
- Optionally start external kernel processes when `executable` is configured.

Production traffic still needs concrete Xray and sing-box config renderers. The current `*.plan.json` output is the clean intermediate model those renderers should consume.

## Why Go

Go fits this project because it builds a static server binary, is common in proxy infrastructure, and works naturally with long-running daemons, process supervision, and Linux deployment.

## Quick Start

```bash
go mod tidy
go test ./...
go run ./cmd/nodebridged -config configs/nodebridge.example.json
```

## Linux Install for Xboard

Run the one-line installer as root:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/RHCloud1/BridgeX-Node/master/deploy/install.sh)
```

The installer works like a V2bX-style wizard. It asks for panel URL, API key, node ID, node protocol, API version, listen IP, TLS certificate paths, and optional subscription URL, then writes `/etc/nodebridge/config.json` automatically. The default panel type is `xboard`.

After installation, run `nodebridge` with no arguments to open the interactive manager:

```bash
nodebridge
```

Common direct commands:

```bash
nodebridge status
nodebridge log
nodebridge restart
nodebridge generate
nodebridge update
nodebridge uninstall
```

Notes:

- `nodebridge generate` reruns the config wizard and backs up the old config first.
- `nodebridge update [version]` reinstalls the selected NodeBridge release. Without a version it uses `latest`.
- The default install repository is `RHCloud1/BridgeX-Node`. Override it with `REPO_OWNER` and `REPO_NAME` if needed.

## API Examples

```bash
curl http://127.0.0.1:8088/healthz

curl -H "Authorization: Bearer change-me" \
  http://127.0.0.1:8088/v1/nodes

curl -X POST http://127.0.0.1:8088/v1/import \
  -H "Authorization: Bearer change-me" \
  -H "Content-Type: application/json" \
  -d '{"name":"manual","url":"https://panel.example.com/api/v1/client/subscribe?token=xxx","format":"xboard"}'
```

## Project Layout

```text
cmd/nodebridged/        daemon entrypoint
internal/config/        JSON config loading and validation
internal/domain/        normalized node and source models
internal/subscription/  Xboard/V2Board-style subscription and share-link parser
internal/panel/         panel HTTP client and future panel protocol adapters
internal/service/       API server, in-memory registry, periodic sync
internal/kernel/        external kernel lifecycle and generated plan files
configs/                example runtime config
deploy/                 Docker and systemd deployment assets
docs/                   architecture and deployment notes
```

## Roadmap

1. Add `internal/kernel/renderers/singbox` to translate normalized nodes into real sing-box JSON.
2. Add `internal/kernel/renderers/xray` to translate normalized nodes into real Xray inbound/outbound/routing JSON.
3. Add Xboard node API support for node info, user list, traffic report, and online IP report.
4. Persist registry snapshots to disk or SQLite so restarts keep the last known node state.
5. Add admin UI or CLI for import testing, renderer preview, and hot reload.
