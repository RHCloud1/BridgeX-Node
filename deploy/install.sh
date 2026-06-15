#!/usr/bin/env bash
set -euo pipefail

REPO_OWNER="${REPO_OWNER:-RHCloud1}"
REPO_NAME="${REPO_NAME:-BridgeX-Node}"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/nodebridge}"
CONFIG_DIR="${CONFIG_DIR:-/etc/nodebridge}"
DATA_DIR="${DATA_DIR:-/var/lib/nodebridge}"
SERVICE_FILE="/etc/systemd/system/nodebridge.service"
VERSION="${1:-latest}"
SING_BOX_MINOR="${SING_BOX_MINOR:-1.12}"

red='\033[0;31m'
green='\033[0;32m'
yellow='\033[0;33m'
plain='\033[0m'

OS_ID=""
ARCH=""
CORE_TYPE=""
CORE_BIN=""
CORE_CONFIG_FILE=""
RENDERER=""
NODE_TYPE=""
PANELS_JSON=""
HAS_ANYTLS=0

need_root() {
  if [[ "${EUID}" -ne 0 ]]; then
    echo -e "${red}error:${plain} run as root."
    exit 1
  fi
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

ask_yes_no() {
  local prompt="$1"
  local default="${2:-n}"
  local input
  if [[ "${default}" == "y" ]]; then
    read -r -p "${prompt} [Y/n]: " input
    input="${input:-y}"
  else
    read -r -p "${prompt} [y/N]: " input
    input="${input:-n}"
  fi
  case "${input,,}" in
    y|yes) return 0 ;;
    *) return 1 ;;
  esac
}

show_help() {
  cat <<EOF
NodeBridge installer

Usage:
  bash install.sh [version]

Examples:
  bash install.sh
  bash install.sh latest
  bash install.sh v0.1.0

The installer generates ${CONFIG_DIR}/config.json interactively.
Default panel type: xboard.
EOF
}

json_escape() {
  printf '%s' "$1" \
    | sed -e 's/\\/\\\\/g' -e 's/"/\\"/g' -e 's/\x0d//g' -e ':a;N;$!ba;s/\n/\\n/g'
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
    | { grep -E "^v${prefix//./\\.}\\." || true; } \
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
  local choice
  echo "Core type:"
  echo "  1) sing-box (recommended for AnyTLS)"
  echo "  2) xray"
  echo "  3) hysteria2"
  choice="$(ask "Select core" "1")"
  case "${choice}" in
    2)
      CORE_TYPE="xray"
      RENDERER="xray-current"
      CORE_BIN="/usr/local/bin/xray"
      CORE_CONFIG_FILE="${DATA_DIR}/xray-main.json"
      ;;
    3)
      CORE_TYPE="hysteria2"
      RENDERER="hysteria2-current"
      CORE_BIN="/usr/local/bin/hysteria"
      CORE_CONFIG_FILE="${DATA_DIR}/hysteria2-main.yaml"
      ;;
    *)
      CORE_TYPE="sing-box"
      RENDERER="sing-box-1.12"
      CORE_BIN="/usr/local/bin/sing-box"
      CORE_CONFIG_FILE="${DATA_DIR}/sing-main.json"
      ;;
  esac
}

choose_node_type() {
  local choice
  echo "Node protocol:"
  echo "  1) shadowsocks"
  echo "  2) vless"
  echo "  3) vmess"
  echo "  4) trojan"
  echo "  5) hysteria"
  echo "  6) hysteria2"
  echo "  7) anytls"
  echo "  8) tuic"
  choice="$(ask "Select protocol" "7")"
  case "${choice}" in
    1) NODE_TYPE="shadowsocks" ;;
    2) NODE_TYPE="vless" ;;
    3) NODE_TYPE="vmess" ;;
    4) NODE_TYPE="trojan" ;;
    5) NODE_TYPE="hysteria" ;;
    6) NODE_TYPE="hysteria2" ;;
    7) NODE_TYPE="anytls" ;;
    8) NODE_TYPE="tuic" ;;
    *) NODE_TYPE="anytls" ;;
  esac
}

node_type_needs_tls() {
  case "$1" in
    anytls|hysteria|hysteria2|trojan|tuic) return 0 ;;
    *) return 1 ;;
  esac
}

install_sing_box_core() {
  local tag version asset url tmp extract
  tag="$(latest_repo_tag_by_prefix SagerNet/sing-box "${SING_BOX_MINOR}")"
  if [[ -z "${tag}" ]]; then
    echo -e "${yellow}cannot find sing-box ${SING_BOX_MINOR}.x; using latest stable.${plain}"
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

append_panel_json() {
  local panel_json="$1"
  if [[ -z "${PANELS_JSON}" ]]; then
    PANELS_JSON="${panel_json}"
  else
    PANELS_JSON+=$',\n'
    PANELS_JSON+="${panel_json}"
  fi
}

collect_nodes() {
  PANELS_JSON=""
  HAS_ANYTLS=0
  local idx=1
  local reuse_api="n"
  local reuse_api_host=""
  local reuse_api_key=""

  if ask_yes_no "Reuse same panel URL and API key for multiple nodes?" "n"; then
    reuse_api="y"
    reuse_api_host="$(ask "Panel URL" "https://panel.example.com")"
    reuse_api_key="$(ask "API key")"
  fi

  while true; do
    local panel_name api_host api_key node_id api_version listen_ip subscribe_url subscribe_format
    local cert_domain cert_file key_file panel_json
    panel_name="$(ask "Node name" "main-panel${idx}")"
    if [[ "${reuse_api}" == "y" ]]; then
      api_host="${reuse_api_host}"
      api_key="${reuse_api_key}"
    else
      api_host="$(ask "Panel URL, for example https://panel.example.com")"
      api_key="$(ask "API key")"
    fi
    node_id="$(ask "Node ID" "${idx}")"
    if ! [[ "${node_id}" =~ ^[0-9]+$ ]]; then
      echo -e "${red}node_id must be a number.${plain}"
      exit 1
    fi
    api_version="$(ask "Panel API version (v1/v2)" "v1")"
    listen_ip="$(ask "Listen IP" "0.0.0.0")"

    choose_node_type
    if [[ "${NODE_TYPE}" == "anytls" ]]; then
      HAS_ANYTLS=1
    fi

    subscribe_url="$(ask "Subscription URL, leave empty to use panel node API" "")"
    if [[ -n "${subscribe_url}" ]]; then
      subscribe_format="$(ask "Subscription format" "xboard")"
    else
      subscribe_format=""
    fi

    if node_type_needs_tls "${NODE_TYPE}"; then
      cert_domain="$(ask "TLS certificate domain" "example.com")"
      cert_file="$(ask "Certificate file path" "${CONFIG_DIR}/fullchain.cer")"
      key_file="$(ask "Private key file path" "${CONFIG_DIR}/cert.key")"
    else
      cert_domain=""
      cert_file=""
      key_file=""
    fi

    panel_json="$(cat <<EOF
    {
      "name": "$(json_escape "${panel_name}")",
      "type": "xboard",
      "api_version": "$(json_escape "${api_version}")",
      "api_host": "$(json_escape "${api_host}")",
      "api_key": "$(json_escape "${api_key}")",
      "node_id": ${node_id},
      "node_type": "$(json_escape "${NODE_TYPE}")",
      "listen_ip": "$(json_escape "${listen_ip}")",
      "enabled": true,
      "subscribe": {
        "url": "$(json_escape "${subscribe_url}")",
        "format": "$(json_escape "${subscribe_format}")"
      },
      "cert": {
        "mode": "file",
        "domain": "$(json_escape "${cert_domain}")",
        "cert_file": "$(json_escape "${cert_file}")",
        "key_file": "$(json_escape "${key_file}")"
      },
      "headers": {
        "User-Agent": "NodeBridge/0.1"
      }
    }
EOF
)"
    append_panel_json "${panel_json}"

    idx=$((idx + 1))
    if ! ask_yes_no "Add another node?" "n"; then
      break
    fi
  done
}

rewrite_config() {
  local listen token sync_interval request_timeout
  mkdir -p "${CONFIG_DIR}" "${DATA_DIR}"
  listen="$(ask "NodeBridge API listen address" "127.0.0.1:8088")"
  token="$(ask "NodeBridge API token" "change-me-$(date +%s)")"
  sync_interval="$(ask "Sync interval" "60s")"
  request_timeout="$(ask "Request timeout" "15s")"

  collect_nodes

  if [[ "${HAS_ANYTLS}" == "1" && "${CORE_TYPE}" != "sing-box" ]]; then
    echo -e "${yellow}AnyTLS requires sing-box. Switching core to sing-box.${plain}"
    CORE_TYPE="sing-box"
    RENDERER="sing-box-1.12"
    CORE_BIN="/usr/local/bin/sing-box"
    CORE_CONFIG_FILE="${DATA_DIR}/sing-main.json"
  fi

  if [[ -f "${CONFIG_DIR}/config.json" ]]; then
    cp "${CONFIG_DIR}/config.json" "${CONFIG_DIR}/config.json.bak.$(date +%s)"
  fi

  cat > "${CONFIG_DIR}/config.json" <<EOF
{
  "server": {
    "listen": "$(json_escape "${listen}")",
    "token": "$(json_escape "${token}")"
  },
  "log": {
    "level": "info"
  },
  "runtime": {
    "work_dir": "$(json_escape "${DATA_DIR}")",
    "sync_interval": "$(json_escape "${sync_interval}")",
    "request_timeout": "$(json_escape "${request_timeout}")"
  },
  "kernels": [
    {
      "name": "${CORE_TYPE}-main",
      "type": "${CORE_TYPE}",
      "enabled": true,
      "executable": "$(json_escape "${CORE_BIN}")",
      "config_path": "$(json_escape "${CORE_CONFIG_FILE}")",
      "version_policy": "pinned",
      "target_version": "${SING_BOX_MINOR}",
      "renderer": "${RENDERER}",
      "args": [],
      "env": {}
    }
  ],
  "panels": [
${PANELS_JSON}
  ]
}
EOF

  if ask_yes_no "Install or update selected external core now?" "y"; then
    install_selected_core
  fi
}

install_config() {
  if [[ "${NODEBRIDGE_REGENERATE:-0}" == "1" || ! -f "${CONFIG_DIR}/config.json" ]]; then
    choose_core
    rewrite_config
    return
  fi

  if ask_yes_no "Existing config found. Regenerate it?" "n"; then
    choose_core
    rewrite_config
    return
  fi

  echo -e "${yellow}keep existing config.${plain}"
  if ask_yes_no "Install or update external core only?" "n"; then
    choose_core
    install_selected_core
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
  local manager_url="https://raw.githubusercontent.com/${REPO_OWNER}/${REPO_NAME}/master/deploy/nodebridge.sh"
  if curl -fsSL "${manager_url}" -o /usr/bin/nodebridge; then
    chmod +x /usr/bin/nodebridge
  elif [[ -f "${INSTALL_DIR}/deploy/nodebridge.sh" ]]; then
    install -m 0755 "${INSTALL_DIR}/deploy/nodebridge.sh" /usr/bin/nodebridge
  elif [[ -f "deploy/nodebridge.sh" ]]; then
    install -m 0755 "deploy/nodebridge.sh" /usr/bin/nodebridge
  else
    echo -e "${yellow}cannot install nodebridge manager command.${plain}"
  fi
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
  echo -e "${green}done.${plain}"
  echo "- menu: nodebridge"
  echo "- status: nodebridge status"
  echo "- logs: nodebridge log"
  echo "- regenerate config: nodebridge generate"
  echo "- update: nodebridge update"
}

main "$@"
