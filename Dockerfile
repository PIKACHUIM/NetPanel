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
# 阶段 2：构建后端（支持交叉编译到目标架构）
# ─────────────────────────────────────────────
FROM --platform=$BUILDPLATFORM golang:1.24-alpine AS backend-builder

# 接收目标平台参数
ARG TARGETPLATFORM
ARG TARGETARCH
ARG TARGETOS

# 根据目标架构安装对应的 CGO 交叉编译工具链
# amd64 构建机器交叉编译 arm64 需要 musl-cross（含 aarch64-linux-musl-gcc）
RUN apk add --no-cache gcc musl-dev && \
    if [ "$TARGETARCH" = "arm64" ]; then \
        apk add --no-cache musl-cross; \
    fi

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

# 根据目标架构设置交叉编译环境
RUN BUILD_TIME=${BUILD_TIME:-$(date -u '+%Y-%m-%dT%H:%M:%SZ')} && \
    if [ "$TARGETARCH" = "arm64" ]; then \
        export CC=aarch64-linux-musl-gcc; \
    fi && \
    CGO_ENABLED=1 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} go build \
      -ldflags="-s -w -X main.Version=${VERSION} -X main.BuildTime=${BUILD_TIME}" \
      -o /app/netpanel .

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
    && update-ca-certificates

# 创建非特权用户（可选，网络功能需要 root）
# RUN addgroup -S netpanel && adduser -S netpanel -G netpanel

WORKDIR /app

COPY --from=backend-builder /app/netpanel ./netpanel

# 数据目录
VOLUME ["/app/data"]

# 默认端口
EXPOSE 8080

ENV TZ=Asia/Shanghai

ENTRYPOINT ["/app/netpanel"]
CMD ["--port", "8080", "--data", "/app/data"]
