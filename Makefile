.PHONY: all build build-frontend build-backend dev clean install-deps help

# 版本信息
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
LDFLAGS := -s -w -X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME)

# 目录
FRONTEND_DIR := frontend
BACKEND_DIR  := backend
DIST_DIR     := dist

# 默认目标
all: build

## help: 显示帮助信息
help:
	@echo "NetPanel 构建工具"
	@echo ""
	@echo "用法:"
	@sed -n 's/^##//p' $(MAKEFILE_LIST) | column -t -s ':' | sed -e 's/^/ /'

## install-deps: 安装所有依赖
install-deps:
	@echo ">>> 安装前端依赖..."
	cd $(FRONTEND_DIR) && npm ci
	@echo ">>> 下载后端依赖..."
	cd $(BACKEND_DIR) && go mod download

## build-webpage: 构建前端
build-frontend:
	@echo ">>> 构建前端..."
	cd $(FRONTEND_DIR) && npm run build
	@echo ">>> 前端构建完成，输出到 backend/embed/dist/"

## build-backend: 构建后端（当前平台）
build-backend:
	@echo ">>> 构建后端 ($(shell go env GOOS)/$(shell go env GOARCH))..."
	@mkdir -p $(DIST_DIR)
	cd $(BACKEND_DIR) && CGO_ENABLED=1 go build \
		-ldflags="$(LDFLAGS)" \
		-o ../$(DIST_DIR)/netpanel$(if $(filter windows,$(shell go env GOOS)),.exe,) .
	@echo ">>> 后端构建完成: $(DIST_DIR)/netpanel"

## build: 构建前端 + 后端
build: build-frontend build-backend

## dev-webpage: 启动前端开发服务器
dev-frontend:
	@echo ">>> 启动前端开发服务器 (http://localhost:3000)..."
	cd $(FRONTEND_DIR) && npm run dev

## dev-backend: 启动后端开发服务器
dev-backend:
	@echo ">>> 启动后端开发服务器 (http://localhost:8080)..."
	cd $(BACKEND_DIR) && go run .

## dev: 同时启动前后端开发服务器（需要 tmux 或 make -j2）
dev:
	@echo ">>> 同时启动前后端，使用 Ctrl+C 停止..."
	$(MAKE) -j2 dev-frontend dev-backend

## test: 运行后端测试
test:
	@echo ">>> 运行后端测试..."
	cd $(BACKEND_DIR) && go test ./... -v -cover

## lint: 运行代码检查
lint:
	@echo ">>> 运行 Go lint..."
	cd $(BACKEND_DIR) && go vet ./...
	@which golangci-lint > /dev/null 2>&1 && \
		cd $(BACKEND_DIR) && golangci-lint run || \
		echo "提示: 安装 golangci-lint 可获得更完整的检查"

## clean: 清理构建产物
clean:
	@echo ">>> 清理构建产物..."
	rm -rf $(DIST_DIR)
	rm -rf $(BACKEND_DIR)/embed/dist
	@echo ">>> 清理完成"

# ===== 跨平台构建 =====

## build-linux-amd64: 构建 Linux amd64
build-linux-amd64:
	@mkdir -p $(DIST_DIR)
	cd $(BACKEND_DIR) && GOOS=linux GOARCH=amd64 CGO_ENABLED=1 go build \
		-ldflags="$(LDFLAGS)" -o ../$(DIST_DIR)/netpanel-linux-amd64 .

## build-linux-arm64: 构建 Linux arm64（需要交叉编译工具链）
build-linux-arm64:
	@mkdir -p $(DIST_DIR)
	cd $(BACKEND_DIR) && GOOS=linux GOARCH=arm64 CGO_ENABLED=1 \
		CC=aarch64-linux-gnu-gcc go build \
		-ldflags="$(LDFLAGS)" -o ../$(DIST_DIR)/netpanel-linux-arm64 .

## build-windows-amd64: 构建 Windows amd64
build-windows-amd64:
	@mkdir -p $(DIST_DIR)
	cd $(BACKEND_DIR) && GOOS=windows GOARCH=amd64 CGO_ENABLED=1 go build \
		-ldflags="$(LDFLAGS)" -o ../$(DIST_DIR)/netpanel-windows-amd64.exe .

## build-darwin-amd64: 构建 macOS amd64
build-darwin-amd64:
	@mkdir -p $(DIST_DIR)
	cd $(BACKEND_DIR) && GOOS=darwin GOARCH=amd64 CGO_ENABLED=1 go build \
		-ldflags="$(LDFLAGS)" -o ../$(DIST_DIR)/netpanel-darwin-amd64 .

## build-darwin-arm64: 构建 macOS arm64 (Apple Silicon)
build-darwin-arm64:
	@mkdir -p $(DIST_DIR)
	cd $(BACKEND_DIR) && GOOS=darwin GOARCH=arm64 CGO_ENABLED=1 go build \
		-ldflags="$(LDFLAGS)" -o ../$(DIST_DIR)/netpanel-darwin-arm64 .

## build-all: 构建所有平台（需要先构建前端）
build-all: build-frontend build-linux-amd64 build-linux-arm64 build-windows-amd64 build-darwin-amd64 build-darwin-arm64
	@echo ">>> 所有平台构建完成:"
	@ls -lh $(DIST_DIR)/

# ===== EasyTier 下载 =====
EASYTIER_VERSION ?= 1.2.1

## download-easytier: 下载当前平台的 EasyTier 二进制
download-easytier:
	@echo ">>> 下载 EasyTier v$(EASYTIER_VERSION)..."
	@mkdir -p $(DIST_DIR)/bin
	@OS=$(shell go env GOOS); ARCH=$(shell go env GOARCH); \
	if [ "$$OS" = "windows" ]; then \
		curl -fsSL "https://github.com/EasyTier/EasyTier/releases/download/v$(EASYTIER_VERSION)/easytier-$$OS-$$ARCH-v$(EASYTIER_VERSION).zip" \
			-o /tmp/easytier.zip && \
		unzip -j /tmp/easytier.zip "easytier-core.exe" -d $(DIST_DIR)/bin/; \
	else \
		curl -fsSL "https://github.com/EasyTier/EasyTier/releases/download/v$(EASYTIER_VERSION)/easytier-$$OS-$$ARCH-v$(EASYTIER_VERSION).tar.gz" \
			-o /tmp/easytier.tar.gz && \
		tar -xzf /tmp/easytier.tar.gz -C $(DIST_DIR)/bin/ --wildcards "*/easytier-core" --strip-components=1 2>/dev/null || \
		tar -xzf /tmp/easytier.tar.gz -C $(DIST_DIR)/bin/ easytier-core 2>/dev/null || true; \
		chmod +x $(DIST_DIR)/bin/easytier-core; \
	fi
	@echo ">>> EasyTier 下载完成: $(DIST_DIR)/bin/"

## run: 构建并运行（开发用）
run: build
	@echo ">>> 启动 NetPanel..."
	./$(DIST_DIR)/netpanel$(if $(filter windows,$(shell go env GOOS)),.exe,)
