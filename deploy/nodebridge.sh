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
  nodebridge            - 打开交互式管理菜单
  nodebridge start      - 启动服务
  nodebridge stop       - 停止服务
  nodebridge restart    - 重启服务
  nodebridge status     - 查看服务状态
  nodebridge log        - 查看实时日志
  nodebridge enable     - 设置开机自启
  nodebridge disable    - 取消开机自启
  nodebridge config     - 编辑配置并重启
  nodebridge install [版本] - 安装/重装程序
  nodebridge update [版本] - 更新程序（默认最新）
  nodebridge generate   - 重新运行安装向导生成配置
  nodebridge uninstall  - 卸载服务和程序
  nodebridge version    - 查看版本
  nodebridge help       - 显示帮助
EOF
}

run_installer() {
  local version="${1:-latest}"
  bash <(curl -fsSL "${INSTALL_SCRIPT_URL}") "${version}"
}

menu() {
  while true; do
    clear || true
    cat <<'EOF'
NodeBridge 管理器
  1) 启动服务
  2) 停止服务
  3) 重启服务
  4) 查看状态
  5) 查看日志
  6) 编辑配置
  7) 重新生成配置
  8) 更新程序
  9) 设置开机自启
 10) 取消开机自启
 11) 查看版本
 12) 卸载
  0) 退出
EOF
    read -r -p "请选择 [0-12]: " choice
    case "${choice}" in
      1) "$0" start ;;
      2) "$0" stop ;;
      3) "$0" restart ;;
      4) "$0" status ;;
      5) "$0" log ;;
      6) "$0" config ;;
      7) "$0" generate ;;
      8) "$0" update ;;
      9) "$0" enable ;;
      10) "$0" disable ;;
      11) "$0" version ;;
      12) "$0" uninstall ;;
      0) exit 0 ;;
      *) echo "无效选择" ;;
    esac
    read -r -p "按回车返回菜单..." _
  done
}

uninstall_nodebridge() {
  need_root
  read -r -p "确认卸载 NodeBridge？这会删除程序和 systemd 服务，但保留 /etc/nodebridge 配置备份 [y/N]: " confirm
  case "${confirm,,}" in
    y|yes) ;;
    *) echo "已取消卸载"; return 0 ;;
  esac
  systemctl stop "${SERVICE}" 2>/dev/null || true
  systemctl disable "${SERVICE}" 2>/dev/null || true
  rm -f "/etc/systemd/system/${SERVICE}.service" /usr/bin/nodebridge
  rm -rf /usr/local/nodebridge
  systemctl daemon-reload
  echo "NodeBridge 已卸载，配置目录仍保留在 /etc/nodebridge"
}

case "${1:-menu}" in
  menu)
    menu
    ;;
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
  enable)
    need_root
    systemctl enable "${SERVICE}"
    ;;
  disable)
    need_root
    systemctl disable "${SERVICE}"
    ;;
  config)
    need_root
    "${EDITOR:-vi}" "${CONFIG_FILE}"
    systemctl restart "${SERVICE}"
    ;;
  install)
    need_root
    run_installer "${2:-latest}"
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
  uninstall)
    uninstall_nodebridge
    ;;
  help|-h|--help)
    usage
    ;;
  *)
    usage
    exit 1
    ;;
esac
