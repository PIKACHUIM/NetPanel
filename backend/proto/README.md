# gRPC Proto 文件

本目录包含监控系统 Agent 与 Server 之间的 gRPC 通信协议定义。

## 文件说明

- `monitor.proto` - gRPC 服务定义和消息格式
- `generate.ps1` - Windows 编译脚本
- `generate.sh` - Linux/macOS 编译脚本

## 安装依赖

### 1. 安装 Protocol Buffers 编译器

**Windows:**
```powershell
# 下载并安装 protoc
# https://github.com/protocolbuffers/protobuf/releases
# 下载 protoc-xx.x-win64.zip，解压后将 bin 目录添加到 PATH
```

**Ubuntu/Debian:**
```bash
sudo apt-get update
sudo apt-get install -y protobuf-compiler
```

**macOS:**
```bash
brew install protobuf
```

### 2. 安装 Go 插件

```bash
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
```

确保 `$GOPATH/bin` 在 PATH 中。

## 编译 Proto 文件

**Windows:**
```powershell
cd backend/proto
./generate.ps1
```

**Linux/macOS:**
```bash
cd backend/proto
chmod +x generate.sh
./generate.sh
```

## 生成的文件

- `monitor.pb.go` - Protocol Buffers 消息定义
- `monitor_grpc.pb.go` - gRPC 服务接口定义

## 服务定义

### MonitorAgent 服务

- `Heartbeat` - 心跳上报（双向流式）
- `ReportMetrics` - 上报监控指标
- `ExecuteCommand` - 执行命令
- `Terminal` - 终端会话（双向流式）
- `TransferFile` - 文件传输（用于脚本下发）

## 使用示例

详见 `backend/service/monitor/grpc_server.go` 和 `backend/cmd/agent/main.go`。
