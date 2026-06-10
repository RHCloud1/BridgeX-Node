# Deployment

## Bare Metal Linux

Install Go on the build machine or CI runner:

```bash
wget https://go.dev/dl/go1.22.12.linux-amd64.tar.gz
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf go1.22.12.linux-amd64.tar.gz
export PATH=/usr/local/go/bin:$PATH
```

Build NodeBridge:

```bash
cd /opt/nodebridge
go test ./...
go build -o bin/nodebridged ./cmd/nodebridged
```

Install:

```bash
sudo mkdir -p /etc/nodebridge /var/lib/nodebridge
sudo cp bin/nodebridged /usr/local/bin/nodebridged
sudo cp configs/nodebridge.example.json /etc/nodebridge/config.json
sudo cp deploy/systemd/nodebridge.service /etc/systemd/system/nodebridge.service
sudo systemctl daemon-reload
sudo systemctl enable --now nodebridge
```

When sing-box or Xray is installed, set the matching `kernels[].executable` path in `/etc/nodebridge/config.json`.

## Docker Compose

```bash
cd /opt/nodebridge
docker compose -f deploy/docker/docker-compose.yml up -d --build
```

Mount real sing-box or Xray binaries into the container only after renderer support is implemented. Until then, keep `executable` empty to render normalized plans only.

## Runtime Paths

- Config: `/etc/nodebridge/config.json`
- Generated plans: `/var/lib/nodebridge/*.plan.json`
- API: default `127.0.0.1:8088`

Expose the API through a private network, VPN, or reverse proxy with authentication. Keep the bearer token private.

