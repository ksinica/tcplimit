package tcplimit

import (
	"fmt"
	"net"

	"golang.org/x/time/rate"
)

// LmitedListener is a wrapper on the net.Listener that accepts
// bandwidth-limited connection.
type LimitedListener interface {
	net.Listener

	// SetLimits set both global and local (per-connection) bandwidth
	// limits in bytes per second. Returns an error, when one of
	// the limits cannot be set.
	SetLimits(global, local int) error
}

type limitedListener struct {
	net.Listener

	limiter *Limiter
}

func (l *limitedListener) Accept() (net.Conn, error) {
	conn, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	return l.limiter.LimitConn(conn), nil
}

func (l *limitedListener) SetLimits(global, local int) error {
	if err := l.limiter.SetGlobalLimit(rate.Limit(global)); err != nil {
		return fmt.Errorf("could not set global limit: %w", err)
	}

	if err := l.limiter.SetLocalLimit(rate.Limit(local)); err != nil {
		return fmt.Errorf("could not set local limit: %w", err)
	}

	return nil
}

func NewListener(l net.Listener) LimitedListener {
	return &limitedListener{
		Listener: l,
		limiter:  NewLimiter(),
	}
}
