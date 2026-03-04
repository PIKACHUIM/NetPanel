//go:build windows

package main

// 使用 rsrc 工具将 Windows manifest 编译为 .syso 资源文件。
// .syso 文件会被 Go 编译器自动链接进最终的 .exe，
// 使 Windows 在启动程序时自动弹出 UAC 提权对话框（requireAdministrator）。
//
// 首次使用前需安装 rsrc 工具：
//   go install github.com/akavel/rsrc@latest
//
// 然后在 backend/ 目录下执行：
//   go generate ./...
//
//go:generate rsrc -manifest netpanel.manifest -o netpanel_windows.syso
