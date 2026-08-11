# gRPC Proto 编译脚本（Windows PowerShell）
# 需要先安装 protoc 和 Go 插件：
# 1. 安装 protoc: https://github.com/protocolbuffers/protobuf/releases
# 2. go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
# 3. go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

Write-Host "开始编译 gRPC Proto 文件..." -ForegroundColor Green

# 检查 protoc 是否安装
if (-not (Get-Command protoc -ErrorAction SilentlyContinue)) {
    Write-Host "错误: protoc 未安装，请先安装 Protocol Buffers 编译器" -ForegroundColor Red
    Write-Host "下载地址: https://github.com/protocolbuffers/protobuf/releases" -ForegroundColor Yellow
    exit 1
}

# 编译 proto 文件
protoc --go_out=. --go_opt=paths=source_relative `
       --go-grpc_out=. --go-grpc_opt=paths=source_relative `
       monitor.proto

if ($LASTEXITCODE -eq 0) {
    Write-Host "Proto 文件编译成功！" -ForegroundColor Green
    Write-Host "生成文件:" -ForegroundColor Cyan
    Write-Host "  - monitor.pb.go (消息定义)" -ForegroundColor Cyan
    Write-Host "  - monitor_grpc.pb.go (gRPC 服务定义)" -ForegroundColor Cyan
} else {
    Write-Host "Proto 文件编译失败！" -ForegroundColor Red
    exit 1
}
