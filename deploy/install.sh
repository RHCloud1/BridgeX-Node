#!/usr/bin/env bash
set -euo pipefail

REPO_OWNER="${REPO_OWNER:-RHCloud1}"
REPO_NAME="${REPO_NAME:-BridgeX-Node}"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/nodebridge}"
CONFIG_DIR="${CONFIG_DIR:-/etc/nodebridge}"
DATA_DIR="${DATA_DIR:-/var/lib/nodebridge}"
SERVICE_FILE="/etc/systemd/system/nodebridge.service"
VERSION="${1:-latest}"

red='\033[0;31m'
green='\033[0;32m'
yellow='\033[0;33m'
plain='\033[0m'

need_root() {
  if [[ "${EUID}" -ne 0 ]]; then
    echo -e "${red}error:${plain} run this script as root."
    exit 1
  fi
}

show_help() {
  cat <<EOF
NodeBridge 一键安装脚本

Usage:
  bash install.sh [version]

Examples:
  bash install.sh
  bash install.sh latest
  bash install.sh v0.1.0

环境变量可覆盖:
  REPO_OWNER, REPO_NAME, INSTALL_DIR, CONFIG_DIR, DATA_DIR
EOF
}

ask() {
  local prompt="$1"
  local default="${2:-}"
  local answer
  if [[ -n "${default}" ]]; then
    read -r -p "${prompt} [${default}]: " answer
    echo "${answer:-$default}"
  else
    read -r -p "${prompt}: " answer
    echo "${answer}"
  fi
}

detect_os() {
  if [[ -f /etc/os-release ]]; then
    . /etc/os-release
    OS_ID="${ID}"
  else
    echo -e "${red}cannot detect linux distribution.${plain}"
    exit 1
  fi
}

detect_arch() {
  case "$(uname -m)" in
    x86_64|amd64) ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    *) echo -e "${red}unsupported arch: $(uname -m)${plain}"; exit 1 ;;
  esac
}

install_base() {
  case "${OS_ID}" in
    debian|ubuntu)
      apt-get update -y
      apt-get install -y curl wget tar unzip ca-certificates
      ;;
    centos|rhel|rocky|almalinux|fedora)
      yum install -y curl wget tar unzip ca-certificates
      ;;
    alpine)
      apk add --no-cache curl wget tar unzip ca-certificates
      ;;
    arch)
      pacman -Sy --noconfirm --needed curl wget tar unzip ca-certificates
      ;;
    *)
      echo -e "${yellow}unknown distribution ${OS_ID}; skip base package install.${plain}"
      ;;
  esac
}

latest_release() {
  curl -fsSL "https://api.github.com/repos/${REPO_OWNER}/${REPO_NAME}/releases/latest" \
    | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' \
    | head -n 1
}

download_nodebridge() {
  mkdir -p "${INSTALL_DIR}" "${CONFIG_DIR}" "${DATA_DIR}"
  local tag="${VERSION}"
  if [[ "${tag}" == "latest" ]]; then
    tag="$(latest_release)"
  fi
  if [[ -z "${tag}" ]]; then
    echo -e "${red}cannot find latest release. Try: bash install.sh v0.1.0${plain}"
    exit 1
  fi

  local asset="nodebridge-linux-${ARCH}.tar.gz"
  local url="https://github.com/${REPO_OWNER}/${REPO_NAME}/releases/download/${tag}/${asset}"
  echo -e "${green}installing NodeBridge ${tag} (${ARCH})${plain}"
  curl -fL "${url}" -o "/tmp/${asset}"
  tar -xzf "/tmp/${asset}" -C "${INSTALL_DIR}"
  rm -f "/tmp/${asset}"
  chmod +x "${INSTALL_DIR}/nodebridged"
}

choose_core() {
  echo "Core type:"
  echo "  1) sing-box"
  echo "  2) xray"
  echo "  3) hysteria2"
  local choice
  choice="$(ask "Select core" "1")"
  case "${choice}" in
    2) CORE_TYPE="xray"; RENDERER="xray-current"; CORE_BIN="/usr/local/bin/xray"; CONFIG_FILE="${DATA_DIR}/xray-main.json" ;;
    3) CORE_TYPE="hysteria2"; RENDERER="hysteria2-current"; CORE_BIN="/usr/local/bin/hysteria"; CONFIG_FILE="${DATA_DIR}/hysteria2-main.yaml" ;;
    *) CORE_TYPE="sing-box"; RENDERER="sing-box-1.12"; CORE_BIN="/usr/local/bin/sing-box"; CONFIG_FILE="${DATA_DIR}/sing-main.json" ;;
  esac
}

choose_node_type() {
  echo "Node protocol:"
  echo "  1) shadowsocks"
  echo "  2) vless"
  echo "  3) vmess"
  echo "  4) trojan"
  echo "  5) hysteria2"
  echo "  6) anytls"
  local choice
  choice="$(ask "Select protocol" "6")"
  case "${choice}" in
    1) NODE_TYPE="shadowsocks" ;;
    2) NODE_TYPE="vless" ;;
    3) NODE_TYPE="vmess" ;;
    4) NODE_TYPE="trojan" ;;
    5) NODE_TYPE="hysteria2" ;;
    *) NODE_TYPE="anytls" ;;
  esac
}

latest_repo_tag() {
  local repo="$1"
  curl -fsSL "https://api.github.com/repos/${repo}/releases/latest" \
    | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' \
    | head -n 1
}

latest_repo_tag_by_prefix() {
  local repo="$1"
  local prefix="$2"
  curl -fsSL "https://api.github.com/repos/${repo}/releases?per_page=100" \
    | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' \
    | grep -E "^v${prefix//./\\.}\\." \
    | head -n 1
}

install_sing_box_core() {
  local minor="${SING_BOX_MINOR:-1.12}"
  local tag version asset url tmp extract
  tag="$(latest_repo_tag_by_prefix SagerNet/sing-box "${minor}")"
  if [[ -z "${tag}" ]]; then
    echo -e "${yellow}cannot find sing-box ${minor}.x; falling back to latest stable.${plain}"
    tag="$(latest_repo_tag SagerNet/sing-box)"
  fi
  version="${tag#v}"
  asset="sing-box-${version}-linux-${ARCH}.tar.gz"
  url="https://github.com/SagerNet/sing-box/releases/download/${tag}/${asset}"
  tmp="/tmp/${asset}"
  extract="/tmp/nodebridge-sing-box"
  echo -e "${green}installing sing-box ${tag}${plain}"
  rm -rf "${extract}"
  mkdir -p "${extract}"
  curl -fL "${url}" -o "${tmp}"
  tar -xzf "${tmp}" -C "${extract}"
  install -m 0755 "$(find "${extract}" -type f -name sing-box | head -n 1)" /usr/local/bin/sing-box
  rm -rf "${tmp}" "${extract}"
}

install_xray_core() {
  local tag xarch asset url tmp extract
  tag="$(latest_repo_tag XTLS/Xray-core)"
  case "${ARCH}" in
    amd64) xarch="64" ;;
    arm64) xarch="arm64-v8a" ;;
    *) xarch="64" ;;
  esac
  asset="Xray-linux-${xarch}.zip"
  url="https://github.com/XTLS/Xray-core/releases/download/${tag}/${asset}"
  tmp="/tmp/${asset}"
  extract="/tmp/nodebridge-xray"
  echo -e "${green}installing Xray ${tag}${plain}"
  rm -rf "${extract}"
  mkdir -p "${extract}"
  curl -fL "${url}" -o "${tmp}"
  unzip -o "${tmp}" -d "${extract}" >/dev/null
  install -m 0755 "${extract}/xray" /usr/local/bin/xray
  rm -rf "${tmp}" "${extract}"
}

install_hysteria2_core() {
  local tag asset url
  tag="$(latest_repo_tag apernet/hysteria)"
  asset="hysteria-linux-${ARCH}"
  url="https://github.com/apernet/hysteria/releases/download/${tag}/${asset}"
  echo -e "${green}installing Hysteria ${tag}${plain}"
  curl -fL "${url}" -o /usr/local/bin/hysteria
  chmod +x /usr/local/bin/hysteria
}

install_selected_core() {
  case "${CORE_TYPE}" in
    sing-box) install_sing_box_core ;;
    xray) install_xray_core ;;
    hysteria2) install_hysteria2_core ;;
  esac
}

generate_config() {
  local api_host api_key node_id api_version cert_domain cert_file key_file token listen
  echo -e "${yellow}NodeBridge first-run config wizard${plain}"
  api_host="$(ask "Panel URL, for example https://panel.example.com")"
  api_key="$(ask "Panel server API key")"
  node_id="$(ask "Node ID" "1")"
  api_version="$(ask "Panel API version: v1 for V2Board/XBoard compatible, v2 for new XBoard" "v1")"
  listen="$(ask "NodeBridge API listen address" "127.0.0.1:8088")"
  token="$(ask "NodeBridge local API token" "change-me-$(date +%s)")"
  choose_core
  choose_node_type
  cert_domain="$(ask "TLS certificate domain" "example.com")"
  cert_file="$(ask "Certificate file path" "${CONFIG_DIR}/fullchain.cer")"
  key_file="$(ask "Private key file path" "${CONFIG_DIR}/cert.key")"

  if [[ -f "${CONFIG_DIR}/config.json" ]]; then
    cp "${CONFIG_DIR}/config.json" "${CONFIG_DIR}/config.json.bak.$(date +%s)"
  fi

  cat > "${CONFIG_DIR}/config.json" <<EOF
{
  "server": {
    "listen": "${listen}",
    "token": "${token}"
  },
  "log": {
    "level": "info"
  },
  "runtime": {
    "work_dir": "${DATA_DIR}",
    "sync_interval": "60s",
    "request_timeout": "15s"
  },
  "kernels": [
    {
      "name": "${CORE_TYPE}-main",
      "type": "${CORE_TYPE}",
      "enabled": true,
      "executable": "${CORE_BIN}",
      "config_path": "${CONFIG_FILE}",
      "version_policy": "pinned",
      "target_version": "1.12",
      "renderer": "${RENDERER}",
      "args": [],
      "env": {}
    }
  ],
  "panels": [
    {
      "name": "main-panel",
      "type": "xboard",
      "api_version": "${api_version}",
      "api_host": "${api_host}",
      "api_key": "${api_key}",
      "node_id": ${node_id},
      "node_type": "${NODE_TYPE}",
      "listen_ip": "0.0.0.0",
      "enabled": true,
      "subscribe": {
        "url": "",
        "format": ""
      },
      "cert": {
        "mode": "file",
        "domain": "${cert_domain}",
        "cert_file": "${cert_file}",
        "key_file": "${key_file}"
      },
      "headers": {
        "User-Agent": "NodeBridge/0.1"
      }
    }
  ]
}
EOF
  install_selected_core
}

install_config() {
  if [[ "${NODEBRIDGE_REGENERATE:-0}" == "1" ]]; then
    generate_config
    return
  fi
  if [[ -f "${CONFIG_DIR}/config.json" ]]; then
    local regen
    regen="$(ask "Existing config found. Regenerate it" "n")"
    if [[ "${regen}" =~ ^[Yy]$ ]]; then
      generate_config
    fi
  else
    generate_config
  fi
}

install_service() {
  cat > "${SERVICE_FILE}" <<EOF
[Unit]
Description=NodeBridge proxy control plane
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=root
ExecStart=${INSTALL_DIR}/nodebridged -config ${CONFIG_DIR}/config.json
WorkingDirectory=${DATA_DIR}
Restart=on-failure
RestartSec=3
LimitNOFILE=1048576

[Install]
WantedBy=multi-user.target
EOF
  systemctl daemon-reload
  systemctl enable nodebridge
}

install_manager() {
  if [[ -f "${INSTALL_DIR}/deploy/nodebridge.sh" ]]; then
    cp "${INSTALL_DIR}/deploy/nodebridge.sh" /usr/bin/nodebridge
  else
    cp deploy/nodebridge.sh /usr/bin/nodebridge
  fi
  chmod +x /usr/bin/nodebridge
}

main() {
  if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
    show_help
    return 0
  fi
  need_root
  detect_os
  detect_arch
  install_base
  download_nodebridge
  install_config
  install_service
  install_manager
  systemctl restart nodebridge || true
  echo -e "${green}done. Use: nodebridge status | nodebridge log | nodebridge config | nodebridge update${plain}"
}

main "$@"
