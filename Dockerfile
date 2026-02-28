# ─────────────────────────────────────────────
# 阶段 1：构建前端
# ─────────────────────────────────────────────
FROM node:20-alpine AS frontend-builder

WORKDIR /app/webpage
COPY webpage/package*.json ./
RUN npm ci --prefer-offline

COPY webpage/ ./
RUN npm run build

# ─────────────────────────────────────────────
# 阶段 2：构建后端
# ─────────────────────────────────────────────
FROM golang:1.22-alpine AS backend-builder

# 安装 CGO 依赖（sqlite 需要 gcc）
RUN apk add --no-cache gcc musl-dev

WORKDIR /app

# 复制 go.mod / go.sum 先缓存依赖
COPY backend/go.mod backend/go.sum ./backend/

WORKDIR /app/backend
RUN go mod download

# 复制源码
COPY backend/ ./

# 复制前端构建产物到 embed 目录
COPY --from=frontend-builder /app/webpage/dist/ ./embed/dist/

ARG VERSION=docker
ARG BUILD_TIME

RUN BUILD_TIME=${BUILD_TIME:-$(date -u '+%Y-%m-%dT%H:%M:%SZ')} && \
    CGO_ENABLED=1 go build \
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
