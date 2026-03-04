package nps

import (
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
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

// RunServerProcess 在子进程模式下运行 NPS 服务端，直到收到退出信号。
// 此函数由主进程通过 exec.Cmd 以 --nps-server 子命令调用，
// NPS 库内部的 os.Exit() 只会终止本子进程，不影响主进程。
func RunServerProcess(confDir string) {
	// 设置 NPS 运行路径
	common.ConfPath = confDir

	// 关闭 NPS 内部日志输出
	logs.Init("off", "info", "", 5, 10, 7, false, false)

	// 加载 beego 配置
	confFile := filepath.Join(confDir, "nps.conf")
	if err := beego.LoadAppConfig("ini", confFile); err != nil {
		os.Exit(1)
	}

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

	// 启动服务端（StartNewServer 会阻塞）
	go server.StartNewServer(bridgePort, task, bridgeType, timeout)

	// 等待初始化完成
	time.Sleep(200 * time.Millisecond)

	// 等待退出信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	// 停止所有服务
	server.RunList.Range(func(key, value interface{}) bool {
		if id, ok := key.(int); ok {
			_ = server.StopServer(id)
		}
		return true
	})
}
