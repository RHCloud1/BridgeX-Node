#!/usr/bin/env bash
set -euo pipefail

SERVICE="nodebridge"
CONFIG_FILE="/etc/nodebridge/config.json"
INSTALL_SCRIPT_URL="${INSTALL_SCRIPT_URL:-https://raw.githubusercontent.com/RHCloud1/BridgeX-Node/master/deploy/install.sh}"

need_root() {
  if [[ "${EUID}" -ne 0 ]]; then
    echo "请以 root 身份运行 nodebridge 管理脚本（systemd 操作需 root）"
    exit 1
  fi
}

usage() {
  cat <<'EOF'
NodeBridge 管理脚本:
  nodebridge start      - 启动服务
  nodebridge stop       - 停止服务
  nodebridge restart    - 重启服务
  nodebridge status     - 查看服务状态
  nodebridge log        - 查看实时日志
  nodebridge config     - 编辑配置并重启
  nodebridge update [版本] - 更新程序（默认最新）
  nodebridge generate   - 重新运行安装向导生成配置
  nodebridge version    - 查看版本
  nodebridge help       - 显示帮助
EOF
}

run_installer() {
  local version="${1:-latest}"
  bash <(curl -fsSL "${INSTALL_SCRIPT_URL}") "${version}"
}

case "${1:-help}" in
  start)
    need_root
    systemctl start "${SERVICE}"
    ;;
  stop)
    need_root
    systemctl stop "${SERVICE}"
    ;;
  restart)
    need_root
    systemctl restart "${SERVICE}"
    ;;
  status)
    systemctl status "${SERVICE}" --no-pager -l || true
    ;;
  log)
    journalctl -u "${SERVICE}" -e --no-pager -f
    ;;
  config)
    need_root
    "${EDITOR:-vi}" "${CONFIG_FILE}"
    systemctl restart "${SERVICE}"
    ;;
  update)
    need_root
    run_installer "${2:-latest}"
    ;;
  generate)
    need_root
    NODEBRIDGE_REGENERATE=1 run_installer latest
    ;;
  version)
    /usr/local/nodebridge/nodebridged --version 2>/dev/null || /usr/local/nodebridge/nodebridged -h
    ;;
  help|-h|--help)
    usage
    ;;
  *)
    usage
    exit 1
    ;;
esac
