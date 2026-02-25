package pmux

import (
	"testing"
	"time"

	"github.com/djylb/nps/lib/logs"
)

func TestPortMux_Close(t *testing.T) {
	logs.Init("stdout", "trace", "", 0, 0, 0, false, true)

	pMux := NewPortMux(8888, "Ds", "Cs")
	go func() {
		if pMux.Start() != nil {
			logs.Warn("Error")
		}
	}()
	time.Sleep(time.Second * 3)
	go func() {
		l := pMux.GetHttpListener()
		conn, err := l.Accept()
		logs.Warn("%v %v", conn, err)
	}()
	go func() {
		l := pMux.GetHttpListener()
		conn, err := l.Accept()
		logs.Warn("%v %v", conn, err)
	}()
	go func() {
		l := pMux.GetHttpListener()
		conn, err := l.Accept()
		logs.Warn("%v %v", conn, err)
	}()
	l := pMux.GetHttpListener()
	conn, err := l.Accept()
	logs.Warn("%v %v", conn, err)
}
