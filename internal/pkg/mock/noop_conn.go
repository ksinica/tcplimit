package mock

import (
	"io"
	"net"
	"time"
)

type noopConn struct {
	q chan struct{}
}

func (c *noopConn) Write(p []byte) (int, error) {
	select {
	case <-c.q:
		return 0, io.EOF
	default:
		return len(p), nil
	}
}

func (c *noopConn) Read(p []byte) (int, error) {
	select {
	case <-c.q:
		return 0, io.EOF
	default:
		return len(p), nil
	}
}

func (c *noopConn) Close() error {
	close(c.q)
	return nil
}

func (*noopConn) LocalAddr() net.Addr {
	return nil
}

func (*noopConn) RemoteAddr() net.Addr {
	return nil
}

func (*noopConn) SetDeadline(time.Time) error {
	return nil
}

func (*noopConn) SetReadDeadline(time.Time) error {
	return nil
}

func (*noopConn) SetWriteDeadline(time.Time) error {
	return nil
}

// NewNoopConn returns net.Conn that basically does nothing.
func NewNoopConn() net.Conn {
	return &noopConn{q: make(chan struct{})}
}
