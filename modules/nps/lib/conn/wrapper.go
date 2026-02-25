package conn

import (
	"fmt"
	"io"
	"net"
	"time"
)

type wrappedConn struct {
	rwc    io.ReadWriteCloser
	parent net.Conn
}

func WrapConn(rwc io.ReadWriteCloser, parent net.Conn) net.Conn {
	return &wrappedConn{rwc: rwc, parent: parent}
}

func (w *wrappedConn) Read(b []byte) (int, error) {
	return w.rwc.Read(b)
}

func (w *wrappedConn) Write(b []byte) (int, error) {
	return w.rwc.Write(b)
}

func (w *wrappedConn) Close() error {
	_ = w.rwc.Close()
	return w.parent.Close()
}

func (w *wrappedConn) LocalAddr() net.Addr {
	return w.parent.LocalAddr()
}

func (w *wrappedConn) RemoteAddr() net.Addr {
	return w.parent.RemoteAddr()
}

func (w *wrappedConn) SetDeadline(t time.Time) error {
	return w.parent.SetDeadline(t)
}

func (w *wrappedConn) SetReadDeadline(t time.Time) error {
	return w.parent.SetReadDeadline(t)
}

func (w *wrappedConn) SetWriteDeadline(t time.Time) error {
	return w.parent.SetWriteDeadline(t)
}

type AddrOverrideConn struct {
	net.Conn
	lAddr net.Addr
	rAddr net.Addr
}

func NewAddrOverrideConn(base net.Conn, remote, local string) (*AddrOverrideConn, error) {
	if base == nil {
		return nil, fmt.Errorf("base conn is nil")
	}
	rAddr, err := parseTCPAddrMaybe(remote)
	if err != nil {
		return nil, fmt.Errorf("invalid remote addr %q: %w", remote, err)
	}
	lAddr, _ := parseTCPAddrMaybe(local)
	return &AddrOverrideConn{
		Conn:  base,
		lAddr: lAddr,
		rAddr: rAddr,
	}, nil
}

func NewAddrOverrideFromAddr(base net.Conn, remote, local net.Addr) *AddrOverrideConn {
	return &AddrOverrideConn{
		Conn:  base,
		lAddr: local,
		rAddr: remote,
	}
}

func (c *AddrOverrideConn) LocalAddr() net.Addr {
	if c.lAddr != nil {
		return c.lAddr
	}
	return c.Conn.LocalAddr()
}

func (c *AddrOverrideConn) RemoteAddr() net.Addr {
	if c.rAddr != nil {
		return c.rAddr
	}
	return c.Conn.RemoteAddr()
}
