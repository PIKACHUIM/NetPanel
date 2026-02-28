.PHONY: all build build-frontend build-backend dev clean install-deps help

# 版本信息
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
LDFLAGS := -s -w -X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME)

# 目录
FRONTEND_DIR := webpage
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
	cd $(FRONTEND_DIR) && npm install
	@echo ">>> 下载后端依赖..."
	cd $(BACKEND_DIR) && go mod download

## build-frontend: 构建前端
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

## dev-frontend: 启动前端开发服务器
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

# ===== OpenWrt / Android 构建 =====

## build-openwrt-amd64: 构建 OpenWrt x86_64（静态链接，无 CGO）
build-openwrt-amd64:
	@mkdir -p $(DIST_DIR)
	cd $(BACKEND_DIR) && GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build \
		-ldflags="$(LDFLAGS) -extldflags '-static'" \
		-o ../$(DIST_DIR)/netpanel-openwrt-amd64 .
	@echo ">>> OpenWrt amd64 构建完成"

## build-openwrt-arm64: 构建 OpenWrt arm64（静态链接，无 CGO）
build-openwrt-arm64:
	@mkdir -p $(DIST_DIR)
	cd $(BACKEND_DIR) && GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build \
		-ldflags="$(LDFLAGS) -extldflags '-static'" \
		-o ../$(DIST_DIR)/netpanel-openwrt-arm64 .
	@echo ">>> OpenWrt arm64 构建完成"

## build-openwrt-mipsle: 构建 OpenWrt MIPS little-endian（常见路由器）
build-openwrt-mipsle:
	@mkdir -p $(DIST_DIR)
	cd $(BACKEND_DIR) && GOOS=linux GOARCH=mipsle GOMIPS=softfloat CGO_ENABLED=0 go build \
		-ldflags="$(LDFLAGS) -extldflags '-static'" \
		-o ../$(DIST_DIR)/netpanel-openwrt-mipsle .
	@echo ">>> OpenWrt mipsle 构建完成"

## build-android-arm64: 构建 Android arm64（需要 Android NDK）
build-android-arm64:
	@mkdir -p $(DIST_DIR)
	@if [ -z "$(ANDROID_NDK_HOME)" ]; then \
		echo "错误: 请设置 ANDROID_NDK_HOME 环境变量指向 Android NDK 路径"; exit 1; \
	fi
	cd $(BACKEND_DIR) && GOOS=android GOARCH=arm64 CGO_ENABLED=1 \
		CC=$(ANDROID_NDK_HOME)/toolchains/llvm/prebuilt/linux-x86_64/bin/aarch64-linux-android21-clang \
		go build -ldflags="$(LDFLAGS)" \
		-o ../$(DIST_DIR)/netpanel-android-arm64 .
	@echo ">>> Android arm64 构建完成"

## build-android-amd64: 构建 Android x86_64（模拟器/x86设备）
build-android-amd64:
	@mkdir -p $(DIST_DIR)
	@if [ -z "$(ANDROID_NDK_HOME)" ]; then \
		echo "错误: 请设置 ANDROID_NDK_HOME 环境变量指向 Android NDK 路径"; exit 1; \
	fi
	cd $(BACKEND_DIR) && GOOS=android GOARCH=amd64 CGO_ENABLED=1 \
		CC=$(ANDROID_NDK_HOME)/toolchains/llvm/prebuilt/linux-x86_64/bin/x86_64-linux-android21-clang \
		go build -ldflags="$(LDFLAGS)" \
		-o ../$(DIST_DIR)/netpanel-android-amd64 .
	@echo ">>> Android amd64 构建完成"

## build-openwrt-all: 构建所有 OpenWrt 目标
build-openwrt-all: build-openwrt-amd64 build-openwrt-arm64 build-openwrt-mipsle
	@echo ">>> 所有 OpenWrt 目标构建完成"

# ===== Docker 构建 =====

DOCKER_IMAGE  ?= ghcr.io/$(shell git remote get-url origin 2>/dev/null | sed 's/.*github.com[:/]\(.*\)\.git/\1/' | tr '[:upper:]' '[:lower:]' || echo "netpanel/netpanel")
DOCKER_TAG    ?= $(VERSION)

## docker-build: 构建 Docker 镜像（当前平台）
docker-build:
	@echo ">>> 构建 Docker 镜像 $(DOCKER_IMAGE):$(DOCKER_TAG)..."
	docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) -t $(DOCKER_IMAGE):latest .
	@echo ">>> Docker 镜像构建完成"

## docker-build-multiarch: 构建多架构 Docker 镜像（需要 buildx）
docker-build-multiarch:
	@echo ">>> 构建多架构 Docker 镜像 (linux/amd64, linux/arm64)..."
	docker buildx build \
		--platform linux/amd64,linux/arm64 \
		-t $(DOCKER_IMAGE):$(DOCKER_TAG) \
		-t $(DOCKER_IMAGE):latest \
		--push .
	@echo ">>> 多架构 Docker 镜像推送完成"

## docker-push: 推送 Docker 镜像
docker-push:
	@echo ">>> 推送 Docker 镜像..."
	docker push $(DOCKER_IMAGE):$(DOCKER_TAG)
	docker push $(DOCKER_IMAGE):latest

## docker-up: 使用 docker-compose 启动服务
docker-up:
	@echo ">>> 启动 Docker Compose 服务..."
	docker compose up -d
	@echo ">>> 服务已启动，访问 http://localhost:8080"

## docker-down: 停止 docker-compose 服务
docker-down:
	@echo ">>> 停止 Docker Compose 服务..."
	docker compose down

## docker-logs: 查看 docker-compose 日志
docker-logs:
	docker compose logs -f netpanel

# ===== 系统服务管理 =====

## service-install: 注册系统服务（需要管理员/root 权限）
service-install:
	@echo ">>> 注册 NetPanel 系统服务..."
	./$(DIST_DIR)/netpanel$(if $(filter windows,$(shell go env GOOS)),.exe,) --install-service
	@echo ">>> 服务注册完成，使用 'make service-start' 启动"

## service-uninstall: 卸载系统服务（需要管理员/root 权限）
service-uninstall:
	@echo ">>> 卸载 NetPanel 系统服务..."
	./$(DIST_DIR)/netpanel$(if $(filter windows,$(shell go env GOOS)),.exe,) --uninstall-service

## service-start: 启动系统服务
service-start:
	@echo ">>> 启动 NetPanel 服务..."
	./$(DIST_DIR)/netpanel$(if $(filter windows,$(shell go env GOOS)),.exe,) --start-service

## service-stop: 停止系统服务
service-stop:
	@echo ">>> 停止 NetPanel 服务..."
	./$(DIST_DIR)/netpanel$(if $(filter windows,$(shell go env GOOS)),.exe,) --stop-service

# ===== Inno Setup 打包（仅 Windows）=====

INNO_COMPILER ?= C:\Program Files (x86)\Inno Setup 6\ISCC.exe

## inno-build: 使用 Inno Setup 打包 Windows 安装程序
inno-build: build-windows-amd64
	@echo ">>> 使用 Inno Setup 打包 Windows 安装程序..."
	@if [ ! -f "$(INNO_COMPILER)" ]; then \
		echo "错误: 未找到 Inno Setup 编译器，请安装 Inno Setup 6 或设置 INNO_COMPILER 变量"; exit 1; \
	fi
	"$(INNO_COMPILER)" scripts/setup.iss
	@echo ">>> Windows 安装程序打包完成，输出到 dist/"

# ===== 完整发布构建 =====

## release-all: 构建所有平台 + OpenWrt + Docker（完整发布）
release-all: build-frontend build-all build-openwrt-all docker-build
	@echo ""
	@echo "✅ 所有平台构建完成:"
	@ls -lh $(DIST_DIR)/
	@echo ""
	@echo "Docker 镜像: $(DOCKER_IMAGE):$(DOCKER_TAG)"

.PHONY: build-openwrt-amd64 build-openwrt-arm64 build-openwrt-mipsle \
        build-android-arm64 build-android-amd64 build-openwrt-all \
        docker-build docker-build-multiarch docker-push docker-up docker-down docker-logs \
        service-install service-uninstall service-start service-stop \
        inno-build release-all
