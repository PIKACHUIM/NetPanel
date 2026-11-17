# ─────────────────────────────────────────────
# 阶段 1：构建前端（始终在构建机器原生架构上运行）
# ─────────────────────────────────────────────
FROM --platform=$BUILDPLATFORM node:20-alpine AS frontend-builder

WORKDIR /app/webpage
COPY webpage/package*.json ./
RUN npm ci --prefer-offline

COPY webpage/ ./
RUN npm run build

# ─────────────────────────────────────────────
# xx：Docker 官方交叉编译辅助工具
# ─────────────────────────────────────────────
FROM --platform=$BUILDPLATFORM tonistiigi/xx AS xx

# ─────────────────────────────────────────────
# 阶段 2：构建后端（支持交叉编译到目标架构）
# ─────────────────────────────────────────────
FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS backend-builder

# 接收目标平台参数
ARG TARGETPLATFORM
ARG TARGETARCH
ARG TARGETOS

# 引入 xx 工具（提供 xx-apk、xx-go、xx-cc 等辅助命令）
COPY --from=xx / /

# 安装 clang/lld 作为交叉编译工具链
# clang 是单一二进制，天然支持多架构交叉编译，不依赖外部汇编器
# 可彻底避免 gcc 交叉编译时 runtime/cgo 汇编器（as）架构不匹配的问题
# xx-apk 安装目标架构的 musl-dev 和 gcc，提供 crtbeginS.o / libgcc 等链接所需文件
RUN apk add --no-cache clang lld musl-dev gcc && \
    xx-apk add --no-cache musl-dev gcc

WORKDIR /app

# 复制 go.mod / go.sum 先缓存依赖
COPY backend/go.mod backend/go.sum ./backend/

WORKDIR /app/backend
RUN go mod download

# 复制源码
COPY backend/ ./

# 复制前端构建产物到 embed 目录
# vite.config.ts 中 outDir 为 '../backend/embed/dist'，即输出到 /app/backend/embed/dist
COPY --from=frontend-builder /app/backend/embed/dist/ ./embed/dist/

ARG VERSION=docker
ARG BUILD_TIME

# 使用 xx-go + clang 进行交叉编译
# CC/CXX 指向 xx-clang/xx-clang++，由 xx 工具自动注入目标架构的 --target 参数
# clang 内置汇编器（integrated-as），无需外部 as，彻底解决 runtime/cgo 汇编器架构不匹配问题
RUN BUILD_TIME=${BUILD_TIME:-$(date -u '+%Y-%m-%dT%H:%M:%SZ')} && \
    CC=xx-clang \
    CXX=xx-clang++ \
    CGO_ENABLED=1 xx-go build \
      -ldflags="-s -w -X main.Version=${VERSION} -X main.BuildTime=${BUILD_TIME}" \
      -o /app/netpanel .

# ─────────────────────────────────────────────
# cloudflared 下载阶段
# ─────────────────────────────────────────────
# 把 cloudflared 一并打进镜像，用户无需在面板里手动下载。
# 原因：面板内的"自动下载"依赖 GitHub Releases，国内网络经常超时/失败，
# 导致「装好了面板却用不了 CF 隧道」。预置到镜像后可开箱即用。
#
# 可通过构建参数覆盖版本或改用镜像加速：
#   docker build --build-arg CLOUDFLARED_VERSION=2026.10.7 \
#                --build-arg CLOUDFLARED_MIRROR=https://ghfast.top/ .
FROM --platform=$BUILDPLATFORM alpine:3.19 AS cloudflared-builder

ARG TARGETARCH
ARG CLOUDFLARED_VERSION=2026.10.7
# 留空表示直连 GitHub；国内构建可设为 https://ghfast.top/ 等加速前缀（需以 / 结尾）
ARG CLOUDFLARED_MIRROR=

RUN set -eux; \
    apk add --no-cache curl; \
    case "${TARGETARCH}" in \
      amd64) cf_arch="amd64" ;; \
      arm64) cf_arch="arm64" ;; \
      arm)   cf_arch="arm"   ;; \
      386)   cf_arch="386"   ;; \
      *) echo "unsupported arch: ${TARGETARCH}" >&2; exit 1 ;; \
    esac; \
    url="${CLOUDFLARED_MIRROR}https://github.com/cloudflare/cloudflared/releases/download/${CLOUDFLARED_VERSION}/cloudflared-linux-${cf_arch}"; \
    echo "downloading ${url}"; \
    # --http1.1：部分 CDN/加速镜像对 HTTP/2 支持不佳，会报 curl exit 92；\
    # 多次重试 + 退避，应对国内访问 GitHub 的频繁中断。\
    curl -fsSL --http1.1 --retry 5 --retry-delay 3 --retry-all-errors \
         --connect-timeout 30 -o /tmp/cloudflared "${url}"; \
    chmod +x /tmp/cloudflared; \
    /tmp/cloudflared --version || true; \
    # 严格校验：确保拿到的是可执行的 Linux 二进制而非错误页面
    test -s /tmp/cloudflared

# ─────────────────────────────────────────────
# 阶段 3：最终运行镜像
# ─────────────────────────────────────────────
FROM alpine:3.19

LABEL org.opencontainers.image.title="NetPanel"
LABEL org.opencontainers.image.description="NetPanel - Network Management Panel"
LABEL org.opencontainers.image.source="https://github.com/netpanel/netpanel"

# 安装运行时依赖
RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    iptables \
    ip6tables \
    iproute2 \
    # 中文字体：面板 UI 含大量中文，alpine 默认无中文字形，
    # 缺字会回退到 terminal 字体导致排版错乱
    font-noto-cjk \
    && update-ca-certificates

# 创建非特权用户（可选，网络功能需要 root）
# RUN addgroup -S netpanel && adduser -S netpanel -G netpanel

WORKDIR /app

COPY --from=backend-builder /app/netpanel ./netpanel

# 预置 cloudflared（供 CF 隧道功能直接使用，免去面板内下载）
COPY --from=cloudflared-builder /tmp/cloudflared /usr/local/bin/cloudflared
RUN chmod +x /usr/local/bin/cloudflared && \
    /usr/local/bin/cloudflared --version

# 数据目录
VOLUME ["/app/data"]

# 默认端口
EXPOSE 8080

ENV TZ=Asia/Shanghai

ENTRYPOINT ["/app/netpanel"]
CMD ["--port", "8080", "--data", "/app/data"]

# 健康检查：面板初始化状态接口（公开端点，容器内自检）
HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 \
    CMD wget -qO- http://127.0.0.1:8080/api/v1/init/status >/dev/null 2>&1 || exit 1
