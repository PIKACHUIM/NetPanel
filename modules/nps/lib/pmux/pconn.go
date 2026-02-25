package pmux

import (
	"net"
	"time"
)

type PortConn struct {
	Conn     net.Conn
	rs       []byte
	readMore bool
}

func newPortConn(conn net.Conn, rs []byte, readMore bool) *PortConn {
	return &PortConn{
		Conn:     conn,
		rs:       rs,
		readMore: readMore,
	}
}
func (pConn *PortConn) Read(b []byte) (n int, err error) {
	if pConn.rs != nil {
		if len(pConn.rs) > 0 {
			n := copy(b, pConn.rs)
			pConn.rs = pConn.rs[n:]
			return n, nil
		}
		pConn.rs = nil
	}
	return pConn.Conn.Read(b[n:])
}

func (pConn *PortConn) Write(b []byte) (n int, err error) {
	return pConn.Conn.Write(b)
}

func (pConn *PortConn) Close() error {
	return pConn.Conn.Close()
}

func (pConn *PortConn) LocalAddr() net.Addr {
	return pConn.Conn.LocalAddr()
}

func (pConn *PortConn) RemoteAddr() net.Addr {
	return pConn.Conn.RemoteAddr()
}

func (pConn *PortConn) SetDeadline(t time.Time) error {
	return pConn.Conn.SetDeadline(t)
}

func (pConn *PortConn) SetReadDeadline(t time.Time) error {
	return pConn.Conn.SetReadDeadline(t)
}

func (pConn *PortConn) SetWriteDeadline(t time.Time) error {
	return pConn.Conn.SetWriteDeadline(t)
}
