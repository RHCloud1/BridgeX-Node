# NodeBridge

NodeBridge is a server-side proxy control-plane scaffold for importing panel subscriptions, normalizing nodes, and orchestrating external proxy kernels such as sing-box or Xray.

This repository is intentionally split into small modules so it can grow into a V2Board/V2bX-style node service without binding the whole project to one panel or one kernel.

## Current Scope

- Import common subscription bodies, including classic V2Board base64 line lists.
- Parse common share links: VMess, VLESS, Trojan, Shadowsocks, Hysteria, Hysteria2, TUIC, and AnyTLS.
- Expose a small HTTP API for health, manual import, node listing, and kernel status.
- Periodically sync configured panel subscriptions.
- Render a normalized kernel plan file for each configured core.
- Optionally start external kernel processes when `executable` is configured.

Production traffic still needs concrete Xray and sing-box config renderers. The current `*.plan.json` output is the clean intermediate model those renderers should consume.

## Why Go

Go fits this project because it builds a static server binary, is common in proxy infrastructure, and works naturally with long-running daemons, process supervision, and Linux deployment. This machine did not have Go installed when the scaffold was created, so the project was placed under:

```text
D:\Project\Go\nodebridge
```

## Quick Start

```bash
go mod tidy
go test ./...
go run ./cmd/nodebridged -config configs/nodebridge.example.json
```

API examples:

```bash
curl http://127.0.0.1:8088/healthz

curl -H "Authorization: Bearer change-me" \
  http://127.0.0.1:8088/v1/nodes

curl -X POST http://127.0.0.1:8088/v1/import \
  -H "Authorization: Bearer change-me" \
  -H "Content-Type: application/json" \
  -d '{"name":"manual","url":"https://panel.example.com/api/v1/client/subscribe?token=xxx","format":"v2board"}'
```

## Project Layout

```text
cmd/nodebridged/        daemon entrypoint
internal/config/        JSON config loading and validation
internal/domain/        normalized node and source models
internal/subscription/  V2Board-style subscription and share-link parser
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
3. Add V2Board node API support for node info, user list, traffic report, and online IP report.
4. Persist registry snapshots to disk or SQLite so restarts keep the last known node state.
5. Add admin UI or CLI for import testing, renderer preview, and hot reload.

