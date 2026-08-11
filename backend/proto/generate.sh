#!/bin/bash
# gRPC Proto 编译脚本（Linux/macOS）
# 需要先安装 protoc 和 Go 插件：
# 1. 安装 protoc: apt-get install -y protobuf-compiler 或 brew install protobuf
# 2. go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
# 3. go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

echo "开始编译 gRPC Proto 文件..."

# 检查 protoc 是否安装
if ! command -v protoc &> /dev/null; then
    echo "错误: protoc 未安装，请先安装 Protocol Buffers 编译器"
    echo "Ubuntu/Debian: sudo apt-get install -y protobuf-compiler"
    echo "macOS: brew install protobuf"
    exit 1
fi

# 编译 proto 文件
protoc --go_out=. --go_opt=paths=source_relative \
       --go-grpc_out=. --go-grpc_opt=paths=source_relative \
       monitor.proto

if [ $? -eq 0 ]; then
    echo "Proto 文件编译成功！"
    echo "生成文件:"
    echo "  - monitor.pb.go (消息定义)"
    echo "  - monitor_grpc.pb.go (gRPC 服务定义)"
else
    echo "Proto 文件编译失败！"
    exit 1
fi
