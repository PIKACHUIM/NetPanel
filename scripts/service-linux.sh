#!/bin/bash
# NetPanel Linux Service 管理脚本（基于 systemd）
# 需要 root 权限
# 用法:
#   sudo ./service-linux.sh install   - 安装并启用服务
#   sudo ./service-linux.sh uninstall - 停止并卸载服务
#   sudo ./service-linux.sh start     - 启动服务
#   sudo ./service-linux.sh stop      - 停止服务
#   sudo ./service-linux.sh restart   - 重启服务
#   sudo ./service-linux.sh status    - 查看服务状态
#   sudo ./service-linux.sh logs      - 查看实时日志

set -euo pipefail

SERVICE_NAME="netpanel"
INSTALL_DIR="/opt/netpanel"
BINARY_NAME="netpanel"
BINARY_PATH="${INSTALL_DIR}/${BINARY_NAME}"
DATA_DIR="${INSTALL_DIR}/data"
PORT=8080
SERVICE_FILE="/etc/systemd/system/${SERVICE_NAME}.service"

# ── 颜色输出 ────────────────────────────────────────────────
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

info()    { echo -e "${GREEN}[INFO]${NC} $*"; }
warn()    { echo -e "${YELLOW}[WARN]${NC} $*"; }
error()   { echo -e "${RED}[ERROR]${NC} $*" >&2; }

# ── 检查 root 权限 ──────────────────────────────────────────
assert_root() {
    if [ "$(id -u)" -ne 0 ]; then
        error "请以 root 权限运行此脚本（sudo ./service-linux.sh $1）"
        exit 1
    fi
}

# ── 检查 systemd ────────────────────────────────────────────
assert_systemd() {
    if ! command -v systemctl &>/dev/null; then
        error "未检测到 systemd，请手动配置服务。"
        exit 1
    fi
}

# ── 安装服务 ────────────────────────────────────────────────
do_install() {
    assert_root install
    assert_systemd

    if [ ! -f "${BINARY_PATH}" ]; then
        error "未找到可执行文件: ${BINARY_PATH}"
        echo "请先将 netpanel 二进制文件复制到 ${INSTALL_DIR}/"
        exit 1
    fi

    chmod +x "${BINARY_PATH}"
    mkdir -p "${DATA_DIR}"

    info "写入 systemd 服务文件: ${SERVICE_FILE}"
    cat > "${SERVICE_FILE}" <<EOF
[Unit]
Description=NetPanel - Network Management Panel
Documentation=https://github.com/netpanel/netpanel
After=network-online.target
Wants=network-online.target
StartLimitIntervalSec=60
StartLimitBurst=3

[Service]
Type=simple
User=root
Group=root
WorkingDirectory=${INSTALL_DIR}
ExecStart=${BINARY_PATH} --port ${PORT} --data ${DATA_DIR}
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
    systemctl enable "${SERVICE_NAME}"
    systemctl start  "${SERVICE_NAME}"

    sleep 2
    if systemctl is-active --quiet "${SERVICE_NAME}"; then
        info "✅ 服务安装并启动成功！"
        info "   安装目录: ${INSTALL_DIR}"
        info "   数据目录: ${DATA_DIR}"
        info "   访问地址: http://localhost:${PORT}"
    else
        error "服务启动失败，请查看日志: journalctl -u ${SERVICE_NAME} -n 50"
        exit 1
    fi
}

# ── 卸载服务 ────────────────────────────────────────────────
do_uninstall() {
    assert_root uninstall
    assert_systemd

    if systemctl is-active --quiet "${SERVICE_NAME}" 2>/dev/null; then
        info "正在停止服务..."
        systemctl stop "${SERVICE_NAME}"
    fi

    if systemctl is-enabled --quiet "${SERVICE_NAME}" 2>/dev/null; then
        systemctl disable "${SERVICE_NAME}"
    fi

    if [ -f "${SERVICE_FILE}" ]; then
        rm -f "${SERVICE_FILE}"
        systemctl daemon-reload
    fi

    info "✅ 服务已卸载。"
    warn "安装目录 '${INSTALL_DIR}' 未被删除，如需清理请手动执行: rm -rf ${INSTALL_DIR}"
}

# ── 启动 / 停止 / 重启 ──────────────────────────────────────
do_start() {
    assert_root start
    systemctl start "${SERVICE_NAME}"
    sleep 1
    if systemctl is-active --quiet "${SERVICE_NAME}"; then
        info "✅ 服务已启动，访问 http://localhost:${PORT}"
    else
        error "启动失败，请查看日志: journalctl -u ${SERVICE_NAME} -n 50"
        exit 1
    fi
}

do_stop() {
    assert_root stop
    systemctl stop "${SERVICE_NAME}"
    info "✅ 服务已停止。"
}

do_restart() {
    assert_root restart
    systemctl restart "${SERVICE_NAME}"
    sleep 1
    if systemctl is-active --quiet "${SERVICE_NAME}"; then
        info "✅ 服务已重启，访问 http://localhost:${PORT}"
    else
        error "重启失败，请查看日志: journalctl -u ${SERVICE_NAME} -n 50"
        exit 1
    fi
}

# ── 状态 / 日志 ─────────────────────────────────────────────
do_status() {
    systemctl status "${SERVICE_NAME}" --no-pager || true
}

do_logs() {
    journalctl -u "${SERVICE_NAME}" -f --no-pager
}

# ── 入口 ────────────────────────────────────────────────────
ACTION="${1:-}"
case "${ACTION}" in
    install)   do_install   ;;
    uninstall) do_uninstall ;;
    start)     do_start     ;;
    stop)      do_stop      ;;
    restart)   do_restart   ;;
    status)    do_status    ;;
    logs)      do_logs      ;;
    *)
        echo "用法: sudo $0 {install|uninstall|start|stop|restart|status|logs}"
        exit 1
        ;;
esac
