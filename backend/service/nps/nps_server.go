package nps

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/beego/beego"
	"github.com/djylb/nps/lib/common"
	"github.com/djylb/nps/lib/crypt"
	"github.com/djylb/nps/lib/file"
	"github.com/djylb/nps/lib/logs"
	"github.com/djylb/nps/server"
	"github.com/djylb/nps/server/connection"
	"github.com/djylb/nps/server/tool"
	"github.com/djylb/nps/web/routers"
)

// loadBeegoConfig 加载 beego 配置文件
func loadBeegoConfig(confFile string) error {
	if err := beego.LoadAppConfig("ini", confFile); err != nil {
		return fmt.Errorf("beego 加载配置失败: %w", err)
	}
	return nil
}

// initNpsServer 初始化并启动 NPS 服务端（在 goroutine 中运行，ctx 取消时停止）
// 注意：由于 beego 使用全局配置，同一进程内只能运行一个 NPS 服务端实例
func initNpsServer(ctx context.Context) {
	// 初始化路由
	routers.Init()

	// 初始化 TLS
	cert, ok := common.LoadCert(
		beego.AppConfig.String("bridge_cert_file"),
		beego.AppConfig.String("bridge_key_file"),
	)
	if !ok {
		// 使用随机生成的证书
	}
	crypt.InitTls(cert)

	// 初始化连接服务
	connection.InitConnectionService()

	// 初始化端口允许列表
	tool.InitAllowPort()
	tool.StartSystemInfo()

	// 启动 NPS 服务端
	task := &file.Tunnel{
		Mode: "webServer",
	}

	timeout := beego.AppConfig.DefaultInt("disconnect_timeout", 60)
	bridgePort := connection.BridgePort
	bridgeType := beego.AppConfig.DefaultString("bridge_type", "tcp")
	if bridgeType == "both" {
		bridgeType = "tcp"
	}

	logs.Init("off", "info", "", 5, 10, 7, false, false)

	// 在 goroutine 中启动服务端（StartNewServer 会阻塞）
	go server.StartNewServer(bridgePort, task, bridgeType, timeout)

	// 等待 ctx 取消后停止
	go func() {
		<-ctx.Done()
		// 停止所有服务
		server.RunList.Range(func(key, value interface{}) bool {
			if id, ok := key.(int); ok {
				_ = server.StopServer(id)
			}
			return true
		})
		if server.Bridge != nil {
			// 关闭 bridge
			time.Sleep(500 * time.Millisecond)
		}
	}()

	// 等待服务端初始化完成
	time.Sleep(200 * time.Millisecond)
}

// getNpsConfDir 获取 NPS 服务端配置目录
func getNpsConfDir(dataDir string, id uint) string {
	return filepath.Join(dataDir, "nps", fmt.Sprintf("server_%d", id))
}

// runNpsServer 在当前进程内运行 NPS 服务端（阻塞直到 ctx 取消）
// 注意：由于 beego 使用全局配置，同一进程内只能运行一个 NPS 服务端实例
func runNpsServer(ctx context.Context, confDir string) error {
	// 设置 NPS 运行路径为配置目录
	common.ConfPath = confDir

	// 关闭 NPS 内部日志，避免干扰主程序日志
	logs.Init("off", "info", "", 5, 10, 7, false, false)

	// 加载 beego 配置
	confFile := filepath.Join(confDir, "nps.conf")
	if err := loadBeegoConfig(confFile); err != nil {
		return fmt.Errorf("加载 NPS 配置失败: %w", err)
	}

	// 初始化并启动服务端
	initNpsServer(ctx)

	// 等待 ctx 取消
	<-ctx.Done()
	return nil
}
