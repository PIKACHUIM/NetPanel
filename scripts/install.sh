#!/usr/bin/env bash
# NetPanel 一键安装脚本 (Linux / OpenWrt)
# 用法:
#   curl -fsSL https://raw.githubusercontent.com/YOUR_ORG/netpanel/main/scripts/install.sh | bash
#   bash install.sh [--version v0.1.0] [--port 8080] [--dir /opt/netpanel] [--no-service]
set -euo pipefail

# ─── 颜色输出 ────────────────────────────────────────────────────────────────
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; BLUE='\033[0;34m'; NC='\033[0m'
info()    { echo -e "${BLUE}[INFO]${NC}  $*"; }
success() { echo -e "${GREEN}[OK]${NC}    $*"; }
warn()    { echo -e "${YELLOW}[WARN]${NC}  $*"; }
error()   { echo -e "${RED}[ERROR]${NC} $*" >&2; exit 1; }

# ─── 默认配置 ─────────────────────────────────────────────────────────────────
REPO="${NETPANEL_REPO:-PIKACHUIM/NetPanel}"
VERSION="${NETPANEL_VERSION:-latest}"
INSTALL_DIR="${NETPANEL_DIR:-/opt/netpanel}"
DATA_DIR="${NETPANEL_DATA:-/var/lib/netpanel}"
LOG_DIR="/var/log/netpanel"
PORT="${NETPANEL_PORT:-8080}"
REGISTER_SERVICE=true
SERVICE_NAME="netpanel"
BINARY_NAME="netpanel"

# ─── 镜像源配置 ───────────────────────────────────────────────────────────────
# 优先使用 GitHub 直连，失败后自动回退到国内镜像
# 可通过 NETPANEL_MIRROR 环境变量强制指定镜像源
#   NETPANEL_MIRROR=ghproxy   → 使用 ghproxy 加速
#   NETPANEL_MIRROR=fastgit  → 使用 fastgit 加速
#   NETPANEL_MIRROR=direct   → 仅使用 GitHub 直连（不回退）
MIRROR="${NETPANEL_MIRROR:-auto}"
MIRROR_URLS_GHPROXY="https://ghproxy.com"
MIRROR_URLS_FASTGIT="https://hub.fastgit.xyz"

# ─── 解析参数 ─────────────────────────────────────────────────────────────────
while [[ $# -gt 0 ]]; do
  case "$1" in
    --version)    VERSION="$2";      shift 2 ;;
    --port)       PORT="$2";         shift 2 ;;
    --dir)        INSTALL_DIR="$2";  shift 2 ;;
    --data)       DATA_DIR="$2";     shift 2 ;;
    --no-service) REGISTER_SERVICE=false; shift ;;
    --help|-h)
      echo "用法: install.sh [选项]"
      echo ""
      echo "选项:"
      echo "  --version  <ver>   指定版本，如 v0.1.0（默认: latest）"
      echo "  --port     <port>  监听端口（默认: 8080）"
      echo "  --dir      <path>  安装目录（默认: /opt/netpanel）"
      echo "  --data     <path>  数据目录（默认: /var/lib/netpanel）"
      echo "  --no-service       不注册系统服务"
      echo ""
      echo "环境变量:"
      echo "  NETPANEL_REPO      GitHub 仓库（默认: YOUR_ORG/netpanel）"
      echo "  NETPANEL_VERSION   版本号（默认: latest）"
      echo "  NETPANEL_DIR       安装目录"
      echo "  NETPANEL_DATA      数据目录"
      echo "  NETPANEL_PORT      监听端口"
      exit 0 ;;
    *) warn "未知参数: $1"; shift ;;
  esac
done

# ─── 检查 root 权限 ───────────────────────────────────────────────────────────
check_root() {
  if [[ $EUID -ne 0 ]]; then
    error "请以 root 权限运行此脚本（sudo bash install.sh）"
  fi
}

# ─── 检测架构 ─────────────────────────────────────────────────────────────────
detect_arch() {
  local arch
  arch=$(uname -m)
  case "$arch" in
    x86_64|amd64)          echo "amd64" ;;
    aarch64|arm64)         echo "arm64" ;;
    armv7l|armv7)          echo "armv7" ;;
    armv6l|armv6)          echo "armv6" ;;
    mips64le)              echo "mips64le" ;;
    mips64)                echo "mips64" ;;
    mipsle|mipsel)         echo "mipsle" ;;
    mips)                  echo "mips" ;;
    riscv64)               echo "riscv64" ;;
    *)                     error "不支持的架构: $arch" ;;
  esac
}

# ─── 检测操作系统 ─────────────────────────────────────────────────────────────
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

# ─── 检查依赖 ─────────────────────────────────────────────────────────────────
check_deps() {
  local missing=()
  for cmd in curl; do
    command -v "$cmd" &>/dev/null || missing+=("$cmd")
  done
  if [[ ${#missing[@]} -gt 0 ]]; then
    warn "缺少依赖: ${missing[*]}，尝试自动安装..."
    if command -v apt-get &>/dev/null; then
      apt-get install -y "${missing[@]}" || error "安装依赖失败"
    elif command -v yum &>/dev/null; then
      yum install -y "${missing[@]}" || error "安装依赖失败"
    elif command -v apk &>/dev/null; then
      apk add --no-cache "${missing[@]}" || error "安装依赖失败"
    elif command -v opkg &>/dev/null; then
      opkg update && opkg install "${missing[@]}" || error "安装依赖失败"
    else
      error "无法自动安装依赖，请手动安装: ${missing[*]}"
    fi
  fi
}

# ─── 获取最新版本 ─────────────────────────────────────────────────────────────
get_latest_version() {
  local ver
  ver=$(curl -fsSL --retry 3 --retry-delay 2 \
    "https://api.github.com/repos/${REPO}/releases/latest" \
    | grep '"tag_name"' | head -1 \
    | sed 's/.*"tag_name": *"\([^"]*\)".*/\1/')
  [[ -z "$ver" ]] && error "无法获取最新版本，请使用 --version 手动指定"
  echo "$ver"
}

# ─── 下载辅助：通过镜像源下载 ────────────────────────────────────────────────
try_download() {
  local url="$1" output="$2"
  curl -fsSL --retry 2 --retry-delay 2 --connect-timeout 8 "$url" -o "$output" 2>/dev/null
}

# ─── 下载二进制 ───────────────────────────────────────────────────────────────
download_binary() {
  local arch os_type tmp_file tmp_dir
  arch=$(detect_arch)
  os_type=$(detect_os)

  # OpenWrt 使用专用包名
  local pkg_arch="$arch"
  if [[ "$os_type" == "openwrt" ]]; then
    pkg_arch="${arch}-openwrt"
  fi

  info "检测到系统: ${os_type} / ${arch}"

  if [[ "$VERSION" == "latest" ]]; then
    VERSION=$(get_latest_version)
    info "最新版本: $VERSION"
  fi

  tmp_dir=$(mktemp -d)
  tmp_file="${tmp_dir}/${BINARY_NAME}"

  local github_base="https://github.com/${REPO}/releases/download/${VERSION}"
  local bin_name="netpanel-linux-${pkg_arch}"
  local tgz_name="netpanel-linux-${pkg_arch}.tar.gz"

  info "下载 NetPanel ${VERSION} (linux/${arch})..."

  # 构建下载源列表（按优先级）
  local -a mirror_prefixes=()
  if [[ "$MIRROR" == "ghproxy" ]]; then
    mirror_prefixes=("${MIRROR_URLS_GHPROXY}/")
  elif [[ "$MIRROR" == "fastgit" ]]; then
    mirror_prefixes=("${MIRROR_URLS_FASTGIT}/")
  elif [[ "$MIRROR" != "direct" ]]; then
    # auto: 先直连，再依次尝试镜像
    mirror_prefixes=("" "${MIRROR_URLS_GHPROXY}/" "${MIRROR_URLS_FASTGIT}/")
  fi

  # 尝试下载裸二进制
  for prefix in "${mirror_prefixes[@]}"; do
    local url="${prefix}${github_base}/${bin_name}"
    if [[ -n "$prefix" ]]; then
      info "尝试镜像源: ${url}"
    fi
    if try_download "$url" "$tmp_file"; then
      chmod +x "$tmp_file"
      echo "$tmp_file"
      return
    fi
  done

  # 尝试 tar.gz 格式
  for prefix in "${mirror_prefixes[@]}"; do
    local url="${prefix}${github_base}/${tgz_name}"
    if [[ -n "$prefix" ]]; then
      info "尝试镜像源: ${url}"
    fi
    if try_download "$url" "${tmp_dir}/netpanel.tar.gz"; then
      tar -xzf "${tmp_dir}/netpanel.tar.gz" -C "$tmp_dir" \
        --wildcards "*/netpanel" --strip-components=1 2>/dev/null \
        || tar -xzf "${tmp_dir}/netpanel.tar.gz" -C "$tmp_dir" netpanel 2>/dev/null \
        || error "解压失败: ${url}"
      chmod +x "$tmp_file"
      echo "$tmp_file"
      return
    fi
  done

  error "下载失败，请检查版本号和网络连接。可尝试设置镜像源:\n  NETPANEL_MIRROR=ghproxy curl -fsSL ... | sudo bash"
}

# ─── 安装二进制 ───────────────────────────────────────────────────────────────
install_binary() {
  local tmp_file="$1"

  info "安装到 ${INSTALL_DIR}..."
  mkdir -p "$INSTALL_DIR" "$DATA_DIR" "$LOG_DIR"

  # 备份旧版本
  if [[ -f "${INSTALL_DIR}/${BINARY_NAME}" ]]; then
    local bak_file="${INSTALL_DIR}/${BINARY_NAME}.bak"
    cp "${INSTALL_DIR}/${BINARY_NAME}" "$bak_file"
    warn "已备份旧版本到 ${bak_file}"
  fi

  mv "$tmp_file" "${INSTALL_DIR}/${BINARY_NAME}"
  chmod +x "${INSTALL_DIR}/${BINARY_NAME}"

  # 创建软链接到 /usr/local/bin
  ln -sf "${INSTALL_DIR}/${BINARY_NAME}" /usr/local/bin/netpanel 2>/dev/null || true

  success "二进制文件安装完成: ${INSTALL_DIR}/${BINARY_NAME}"
}

# ─── 写入默认配置 ─────────────────────────────────────────────────────────────
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
register_systemd_service() {
  if ! command -v systemctl &>/dev/null; then
    warn "未检测到 systemd，跳过服务注册"
    return 1
  fi

  info "注册 systemd 服务..."
  cat > "/etc/systemd/system/${SERVICE_NAME}.service" <<EOF
[Unit]
Description=NetPanel - Network Management Panel
Documentation=https://github.com/${REPO}
After=network-online.target
Wants=network-online.target
StartLimitIntervalSec=60
StartLimitBurst=3

[Service]
Type=simple
User=root
Group=root
WorkingDirectory=${INSTALL_DIR}
ExecStart=${INSTALL_DIR}/${BINARY_NAME} --port ${PORT} --data ${DATA_DIR}
ExecReload=/bin/kill -HUP \$MAINPID
Restart=on-failure
RestartSec=5s
KillMode=mixed
KillSignal=SIGTERM
TimeoutStopSec=30
StandardOutput=journal
StandardError=journal
SyslogIdentifier=${SERVICE_NAME}
AmbientCapabilities=CAP_NET_BIND_SERVICE CAP_NET_ADMIN CAP_NET_RAW
CapabilityBoundingSet=CAP_NET_BIND_SERVICE CAP_NET_ADMIN CAP_NET_RAW

[Install]
WantedBy=multi-user.target
EOF

  systemctl daemon-reload
  systemctl enable "$SERVICE_NAME"
  systemctl restart "$SERVICE_NAME"

  sleep 2
  if systemctl is-active --quiet "$SERVICE_NAME"; then
    success "systemd 服务已注册并启动: $SERVICE_NAME"
    info "查看日志: journalctl -u ${SERVICE_NAME} -f"
    return 0
  else
    warn "服务启动失败，请查看日志: journalctl -u ${SERVICE_NAME} -n 50"
    return 1
  fi
}

# ─── 注册 OpenWrt procd 服务 ──────────────────────────────────────────────────
register_openwrt_service() {
  info "注册 OpenWrt procd init.d 服务..."
  cat > "/etc/init.d/${SERVICE_NAME}" <<EOF
#!/bin/sh /etc/rc.common
# NetPanel init.d 服务脚本
USE_PROCD=1
START=99
STOP=10

BINARY="${INSTALL_DIR}/${BINARY_NAME}"
DATA_DIR="${DATA_DIR}"
PORT="${PORT}"

start_service() {
  procd_open_instance
  procd_set_param command "\${BINARY}" --port "\${PORT}" --data "\${DATA_DIR}"
  procd_set_param respawn \${respawn_threshold:-3600} \${respawn_timeout:-5} \${respawn_retry:-5}
  procd_set_param stdout 1
  procd_set_param stderr 1
  procd_set_param pidfile /var/run/${SERVICE_NAME}.pid
  procd_close_instance
}

stop_service() {
  procd_kill ${SERVICE_NAME}
}

reload_service() {
  stop_service
  start_service
}
EOF
  chmod +x "/etc/init.d/${SERVICE_NAME}"
  "/etc/init.d/${SERVICE_NAME}" enable
  "/etc/init.d/${SERVICE_NAME}" start
  success "OpenWrt procd 服务已注册并启动"
}

# ─── 主流程 ───────────────────────────────────────────────────────────────────
main() {
  echo ""
  echo -e "${BLUE}╔══════════════════════════════════════════╗${NC}"
  echo -e "${BLUE}║      NetPanel 一键安装脚本 (Linux)       ║${NC}"
  echo -e "${BLUE}╚══════════════════════════════════════════╝${NC}"
  echo ""

  check_root
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
      register_systemd_service || warn "服务注册失败，可手动运行: ${INSTALL_DIR}/${BINARY_NAME} --port ${PORT} --data ${DATA_DIR}"
    fi
  else
    info "已跳过服务注册（--no-service）"
    info "手动启动: ${INSTALL_DIR}/${BINARY_NAME} --port ${PORT} --data ${DATA_DIR}"
  fi

  # 获取本机 IP
  local ip
  ip=$(hostname -I 2>/dev/null | awk '{print $1}' || echo "localhost")

  echo ""
  echo -e "${GREEN}╔══════════════════════════════════════════════════════╗${NC}"
  echo -e "${GREEN}║  ✅ NetPanel ${VERSION} 安装完成！                   ║${NC}"
  echo -e "${GREEN}║                                                      ║${NC}"
  echo -e "${GREEN}║  访问地址: http://${ip}:${PORT}                      ║${NC}"
  echo -e "${GREEN}║  安装目录: ${INSTALL_DIR}                            ║${NC}"
  echo -e "${GREEN}║  数据目录: ${DATA_DIR}                               ║${NC}"
  echo -e "${GREEN}║                                                      ║${NC}"
  echo -e "${GREEN}║  服务管理（systemd）:                                ║${NC}"
  echo -e "${GREEN}║    状态: systemctl status netpanel                   ║${NC}"
  echo -e "${GREEN}║    启动: systemctl start netpanel                    ║${NC}"
  echo -e "${GREEN}║    停止: systemctl stop netpanel                     ║${NC}"
  echo -e "${GREEN}║    日志: journalctl -u netpanel -f                   ║${NC}"
  echo -e "${GREEN}║                                                      ║${NC}"
  echo -e "${GREEN}║  管理脚本: /usr/local/bin/netpanel                   ║${NC}"
  echo -e "${GREEN}╚══════════════════════════════════════════════════════╝${NC}"
  echo ""
}

main "$@"
