//go:build !windows
// +build !windows

package conn

import (
	"net"

	"github.com/djylb/nps/lib/common"
)

func NewUdpConnByAddr(addr string) (net.PacketConn, error) {
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return nil, err
	}
	port := common.GetPortStrByAddr(addr)

	var conns []net.PacketConn
	if pc4, e4 := net.ListenPacket("udp4", ":"+port); e4 == nil {
		conns = append(conns, pc4)
	}
	if pc6, e6 := net.ListenPacket("udp6", ":"+port); e6 == nil {
		conns = append(conns, pc6)
	}
	if len(conns) == 1 {
		return conns[0], nil
	}
	if len(conns) > 1 {
		return NewSmartUdpConn(conns, udpAddr), nil
	}
	return net.ListenPacket("udp", addr)
}
