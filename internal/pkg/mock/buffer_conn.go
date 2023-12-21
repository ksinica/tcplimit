package mock

import (
	"bytes"
	"net"
	"time"
)

type BufferReturner interface {
	Bytes() []byte
}

type bufferConn struct {
	*bytes.Buffer
}

func (*bufferConn) Close() error {
	return nil
}

func (*bufferConn) LocalAddr() net.Addr {
	return nil
}

func (*bufferConn) RemoteAddr() net.Addr {
	return nil
}

func (*bufferConn) SetDeadline(time.Time) error {
	return nil
}

func (*bufferConn) SetReadDeadline(time.Time) error {
	return nil
}

func (*bufferConn) SetWriteDeadline(time.Time) error {
	return nil
}

func NewBufferConn(buf []byte) net.Conn {
	return &bufferConn{Buffer: bytes.NewBuffer(buf)}
}
