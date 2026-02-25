package server

import (
	"net"
	"net/http"
	"path/filepath"
	"unsafe"

	"github.com/beego/beego"
	"github.com/djylb/nps/bridge"
	"github.com/djylb/nps/lib/common"
	"github.com/djylb/nps/lib/conn"
	"github.com/djylb/nps/lib/logs"
	"github.com/djylb/nps/server/connection"
	"github.com/djylb/nps/server/proxy"
	"github.com/djylb/nps/server/tool"
)

var _ = unsafe.Sizeof(0)

//var httpNum = 0

//go:linkname initBeforeHTTPRun github.com/beego/beego.initBeforeHTTPRun
func initBeforeHTTPRun()

type WebServer struct {
	proxy.BaseServer
	tcpListener net.Listener
}

func (s *WebServer) Start() error {
	ip := connection.WebIp
	p := connection.WebPort

	beego.BConfig.WebConfig.Session.SessionOn = true
	beego.SetStaticPath(beego.AppConfig.String("web_base_url")+"/static", filepath.Join(common.GetRunPath(), "web", "static"))
	beego.SetViewsPath(filepath.Join(common.GetRunPath(), "web", "views"))
	initBeforeHTTPRun()

	if tool.WebServerListener != nil {
		_ = tool.WebServerListener.Close()
		tool.WebServerListener = nil
	}
	lAddr := &net.TCPAddr{IP: net.ParseIP(ip), Port: p}
	tool.WebServerListener = conn.NewVirtualListener(lAddr)

	errCh := make(chan error, 2)

	go func() {
		errCh <- http.Serve(tool.WebServerListener, beego.BeeApp.Handlers)
	}()

	if p > 0 {
		if l, err := connection.GetWebManagerListener(); err == nil {
			s.tcpListener = l
			go func() {
				if beego.AppConfig.String("web_open_ssl") == "true" {
					keyPath := beego.AppConfig.String("web_key_file")
					certPath := beego.AppConfig.String("web_cert_file")
					errCh <- http.ServeTLS(l, beego.BeeApp.Handlers, certPath, keyPath)
				} else {
					errCh <- http.Serve(l, beego.BeeApp.Handlers)
				}
			}()
		} else {
			logs.Error("%v", err)
		}
	} else {
		logs.Info("web_port=0: only virtual listener is active (plain HTTP)")
	}

	return <-errCh
}

func (s *WebServer) Close() error {
	if s.tcpListener != nil {
		_ = s.tcpListener.Close()
	}
	if tool.WebServerListener != nil {
		_ = tool.WebServerListener.Close()
		tool.WebServerListener = nil
	}
	return nil
}

func NewWebServer(bridge *bridge.Bridge) *WebServer {
	s := new(WebServer)
	s.Bridge = bridge
	return s
}
