# One-Click Install Design

The V2bX script repository uses a practical two-layer model:

- `install.sh`: detect OS and architecture, install base packages, download the latest GitHub release, write service files, and install the management command.
- `V2bX.sh`: provide day-two operations such as `start`, `stop`, `restart`, `status`, `log`, `update`, `generate`, and `version`.

NodeBridge follows the same shape:

```bash
wget -N https://raw.githubusercontent.com/your-org/nodebridge/main/deploy/install.sh
bash install.sh
```

After install:

```bash
nodebridge status
nodebridge log
nodebridge config
nodebridge update
nodebridge update v0.1.0
```

## What Is Still Needed

Before this is production-ready, CI must publish release assets:

- `nodebridge-linux-amd64.tar.gz`
- `nodebridge-linux-arm64.tar.gz`

Each archive should contain:

- `nodebridged`
- `configs/nodebridge.example.json`
- `deploy/nodebridge.sh`

The install script already expects that release shape.

## Kernel Downloads

Do not silently chase every latest sing-box release. For a proxy node service, config compatibility is more important than being newest.

Recommended policy:

- `pinned`: use an operator-selected major/minor line, such as `1.12`.
- `stable`: use the latest stable release only after renderer validation passes.
- `manual`: never update kernels automatically.

Every rendered config should be checked with the target kernel before restart:

```bash
sing-box check -c /etc/nodebridge/generated/sing-box.json
xray run -test -config /etc/nodebridge/generated/xray.json
```

