#!/usr/bin/env bash
# NetPanel 一键安装脚本 (Linux)
# 用法: curl -fsSL https://raw.githubusercontent.com/YOUR_ORG/netpanel/main/scripts/install.sh | bash
# 或本地运行: bash install.sh [--version v0.1.0] [--port 8080] [--no-service]
set -euo pipefail

# ─── 颜色输出 ────────────────────────────────────────────────────────────────
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; BLUE='\033[0;34m'; NC='\033[0m'
info()    { echo -e "${BLUE}[INFO]${NC}  $*"; }
success() { echo -e "${GREEN}[OK]${NC}    $*"; }
warn()    { echo -e "${YELLOW}[WARN]${NC}  $*"; }
error()   { echo -e "${RED}[ERROR]${NC} $*" >&2; exit 1; }

# ─── 默认配置 ─────────────────────────────────────────────────────────────────
REPO="YOUR_ORG/netpanel"
VERSION="${NETPANEL_VERSION:-latest}"
INSTALL_DIR="${NETPANEL_DIR:-/opt/netpanel}"
DATA_DIR="${NETPANEL_DATA:-/var/lib/netpanel}"
LOG_DIR="/var/log/netpanel"
PORT="${NETPANEL_PORT:-8080}"
REGISTER_SERVICE=true
SERVICE_NAME="netpanel"
BINARY_NAME="netpanel"

# ─── 解析参数 ─────────────────────────────────────────────────────────────────
while [[ $# -gt 0 ]]; do
  case "$1" in
    --version)  VERSION="$2";        shift 2 ;;
    --port)     PORT="$2";           shift 2 ;;
    --dir)      INSTALL_DIR="$2";    shift 2 ;;
    --no-service) REGISTER_SERVICE=false; shift ;;
    --help|-h)
      echo "用法: install.sh [选项]"
      echo "  --version  <ver>   指定版本，如 v0.1.0（默认: latest）"
      echo "  --port     <port>  监听端口（默认: 8080）"
      echo "  --dir      <path>  安装目录（默认: /opt/netpanel）"
      echo "  --no-service       不注册 systemd 服务"
      exit 0 ;;
    *) warn "未知参数: $1"; shift ;;
  esac
done

# ─── 检测架构 ─────────────────────────────────────────────────────────────────
detect_arch() {
  local arch
  arch=$(uname -m)
  case "$arch" in
    x86_64|amd64)   echo "amd64" ;;
    aarch64|arm64)  echo "arm64" ;;
    armv7l|armv6l)  echo "arm" ;;
    mips*)          echo "mips" ;;
    *)              error "不支持的架构: $arch" ;;
  esac
}

# ─── 检测发行版 ───────────────────────────────────────────────────────────────
detect_os() {
  if [[ -f /etc/openwrt_release ]]; then
    echo "openwrt"
  elif [[ -f /etc/os-release ]]; then
    # shellcheck source=/dev/null
    source /etc/os-release
    echo "${ID:-linux}"
  else
    echo "linux"
  fi
}

# ─── 获取最新版本 ─────────────────────────────────────────────────────────────
get_latest_version() {
  local ver
  ver=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
    | grep '"tag_name"' | head -1 | sed 's/.*"tag_name": *"\([^"]*\)".*/\1/')
  [[ -z "$ver" ]] && error "无法获取最新版本，请使用 --version 手动指定"
  echo "$ver"
}

# ─── 检查依赖 ─────────────────────────────────────────────────────────────────
check_deps() {
  local missing=()
  for cmd in curl tar; do
    command -v "$cmd" &>/dev/null || missing+=("$cmd")
  done
  if [[ ${#missing[@]} -gt 0 ]]; then
    warn "缺少依赖: ${missing[*]}，尝试自动安装..."
    if command -v apt-get &>/dev/null; then
      apt-get install -y "${missing[@]}" || error "安装依赖失败"
    elif command -v yum &>/dev/null; then
      yum install -y "${missing[@]}" || error "安装依赖失败"
    elif command -v opkg &>/dev/null; then
      opkg update && opkg install "${missing[@]}" || error "安装依赖失败"
    else
      error "无法自动安装依赖，请手动安装: ${missing[*]}"
    fi
  fi
}

# ─── 下载二进制 ───────────────────────────────────────────────────────────────
download_binary() {
  local arch os_type download_url tmp_file
  arch=$(detect_arch)
  os_type=$(detect_os)

  # OpenWrt 使用专用包
  if [[ "$os_type" == "openwrt" ]]; then
    arch="${arch}-openwrt"
  fi

  info "检测到系统: ${os_type} / ${arch}"

  if [[ "$VERSION" == "latest" ]]; then
    VERSION=$(get_latest_version)
    info "最新版本: $VERSION"
  fi

  download_url="https://github.com/${REPO}/releases/download/${VERSION}/netpanel-linux-${arch}"
  tmp_file=$(mktemp)

  info "下载 NetPanel ${VERSION} (linux/${arch})..."
  if ! curl -fsSL --progress-bar "$download_url" -o "$tmp_file"; then
    # 尝试 .tar.gz 格式
    download_url="${download_url}.tar.gz"
    curl -fsSL --progress-bar "$download_url" -o "${tmp_file}.tar.gz" \
      || error "下载失败: $download_url"
    tar -xzf "${tmp_file}.tar.gz" -C "$(dirname "$tmp_file")" \
      --wildcards "*/netpanel" --strip-components=1 2>/dev/null \
      || tar -xzf "${tmp_file}.tar.gz" -C "$(dirname "$tmp_file")" netpanel 2>/dev/null \
      || error "解压失败"
    tmp_file="$(dirname "$tmp_file")/netpanel"
    rm -f "${tmp_file}.tar.gz"
  fi

  chmod +x "$tmp_file"
  echo "$tmp_file"
}

# ─── 安装 ─────────────────────────────────────────────────────────────────────
install_binary() {
  local tmp_file="$1"

  info "安装到 ${INSTALL_DIR}..."
  mkdir -p "$INSTALL_DIR" "$DATA_DIR" "$LOG_DIR"

  # 备份旧版本
  if [[ -f "${INSTALL_DIR}/${BINARY_NAME}" ]]; then
    cp "${INSTALL_DIR}/${BINARY_NAME}" "${INSTALL_DIR}/${BINARY_NAME}.bak"
    warn "已备份旧版本到 ${INSTALL_DIR}/${BINARY_NAME}.bak"
  fi

  mv "$tmp_file" "${INSTALL_DIR}/${BINARY_NAME}"
  chmod +x "${INSTALL_DIR}/${BINARY_NAME}"

  # 创建软链接
  ln -sf "${INSTALL_DIR}/${BINARY_NAME}" /usr/local/bin/netpanel 2>/dev/null || true

  success "二进制文件安装完成: ${INSTALL_DIR}/${BINARY_NAME}"
}

# ─── 写入配置 ─────────────────────────────────────────────────────────────────
write_config() {
  local conf_file="${DATA_DIR}/config.yaml"
  if [[ -f "$conf_file" ]]; then
    warn "配置文件已存在，跳过: $conf_file"
    return
  fi

  info "写入默认配置..."
  cat > "$conf_file" <<EOF
# NetPanel 配置文件
server:
  port: ${PORT}
  host: "0.0.0.0"

database:
  path: "${DATA_DIR}/netpanel.db"

log:
  level: "info"
  path: "${LOG_DIR}/netpanel.log"
EOF
  success "配置文件已写入: $conf_file"
}

# ─── 注册 systemd 服务 ────────────────────────────────────────────────────────
register_service() {
  if ! command -v systemctl &>/dev/null; then
    warn "未检测到 systemd，跳过服务注册"
    return
  fi

  info "注册 systemd 服务..."
  cat > "/etc/systemd/system/${SERVICE_NAME}.service" <<EOF
[Unit]
Description=NetPanel Network Manager
After=network.target network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=${INSTALL_DIR}/${BINARY_NAME} --service --port ${PORT} --data ${DATA_DIR}
Restart=on-failure
RestartSec=5s
StandardOutput=journal
StandardError=journal
SyslogIdentifier=${SERVICE_NAME}
WorkingDirectory=${DATA_DIR}

[Install]
WantedBy=multi-user.target
EOF

  systemctl daemon-reload
  systemctl enable "$SERVICE_NAME"
  systemctl restart "$SERVICE_NAME"

  success "服务已注册并启动: $SERVICE_NAME"
  info "查看日志: journalctl -u ${SERVICE_NAME} -f"
}

# ─── OpenWrt init.d 脚本 ──────────────────────────────────────────────────────
register_openwrt_service() {
  info "注册 OpenWrt init.d 服务..."
  cat > "/etc/init.d/${SERVICE_NAME}" <<'EOF'
#!/bin/sh /etc/rc.common
USE_PROCD=1
START=99
STOP=10

start_service() {
  procd_open_instance
  procd_set_param command /opt/netpanel/netpanel --service
  procd_set_param respawn
  procd_set_param stdout 1
  procd_set_param stderr 1
  procd_close_instance
}
EOF
  chmod +x "/etc/init.d/${SERVICE_NAME}"
  "/etc/init.d/${SERVICE_NAME}" enable
  "/etc/init.d/${SERVICE_NAME}" start
  success "OpenWrt 服务已注册并启动"
}

# ─── 主流程 ───────────────────────────────────────────────────────────────────
main() {
  echo ""
  echo -e "${BLUE}╔══════════════════════════════════════╗${NC}"
  echo -e "${BLUE}║      NetPanel 一键安装脚本           ║${NC}"
  echo -e "${BLUE}╚══════════════════════════════════════╝${NC}"
  echo ""

  # 检查 root 权限
  if [[ $EUID -ne 0 ]]; then
    error "请以 root 权限运行此脚本（sudo bash install.sh）"
  fi

  check_deps

  local tmp_file
  tmp_file=$(download_binary)

  install_binary "$tmp_file"
  write_config

  if [[ "$REGISTER_SERVICE" == "true" ]]; then
    local os_type
    os_type=$(detect_os)
    if [[ "$os_type" == "openwrt" ]]; then
      register_openwrt_service
    else
      register_service
    fi
  fi

  echo ""
  echo -e "${GREEN}╔══════════════════════════════════════════════════╗${NC}"
  echo -e "${GREEN}║  ✅ NetPanel 安装完成！                          ║${NC}"
  echo -e "${GREEN}║                                                  ║${NC}"
  echo -e "${GREEN}║  访问地址: http://$(hostname -I | awk '{print $1}'):${PORT}          ║${NC}"
  echo -e "${GREEN}║  数据目录: ${DATA_DIR}                ║${NC}"
  echo -e "${GREEN}║                                                  ║${NC}"
  echo -e "${GREEN}║  服务管理:                                       ║${NC}"
  echo -e "${GREEN}║    启动: systemctl start netpanel                ║${NC}"
  echo -e "${GREEN}║    停止: systemctl stop netpanel                 ║${NC}"
  echo -e "${GREEN}║    日志: journalctl -u netpanel -f               ║${NC}"
  echo -e "${GREEN}╚══════════════════════════════════════════════════╝${NC}"
  echo ""
}

main "$@"
