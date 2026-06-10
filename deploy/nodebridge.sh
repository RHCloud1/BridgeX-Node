#!/usr/bin/env bash
set -euo pipefail

SERVICE="nodebridge"
INSTALL_SCRIPT_URL="${INSTALL_SCRIPT_URL:-https://raw.githubusercontent.com/your-org/nodebridge/main/deploy/install.sh}"

usage() {
  cat <<'EOF'
NodeBridge 管理命令:
  nodebridge start       启动服务
  nodebridge stop        停止服务
  nodebridge restart     重启服务
  nodebridge status      查看状态
  nodebridge log         查看日志
  nodebridge config      编辑配置
  nodebridge update      更新到 latest release
  nodebridge update TAG  更新到指定版本
  nodebridge version     查看版本
  nodebridge generate    重新运行安装脚本并生成配置
EOF
}

case "${1:-}" in
  start) systemctl start "${SERVICE}" ;;
  stop) systemctl stop "${SERVICE}" ;;
  restart) systemctl restart "${SERVICE}" ;;
  status) systemctl status "${SERVICE}" --no-pager -l ;;
  log) journalctl -u "${SERVICE}" -e --no-pager -f ;;
  config)
    "${EDITOR:-vi}" /etc/nodebridge/config.json
    systemctl restart "${SERVICE}"
    ;;
  update)
    version="${2:-latest}"
    bash <(curl -fsSL "${INSTALL_SCRIPT_URL}") "${version}"
    ;;
  generate)
    NODEBRIDGE_REGENERATE=1 bash <(curl -fsSL "${INSTALL_SCRIPT_URL}") latest
    ;;
  version)
    /usr/local/nodebridge/nodebridged --version 2>/dev/null || /usr/local/nodebridge/nodebridged -h
    ;;
  *) usage ;;
esac
