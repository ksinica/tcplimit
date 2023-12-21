package tcplimit

import (
	"net"
	"sync"

	"golang.org/x/time/rate"
)

// Limiter limits the connection's bandwidth. It has two concepts of limits:
//
// 1. global - a limit for all connections, respected by the connections
// independently of their own limits.
// 2. local - a limit specified per connection.
//
// Limiter provides a convenient interface for setting both of the limits,
// giving the ability to shape bandwidth during runtime.
//
// Limiter's methods can be used concurrently.
type Limiter struct {
	globalLimiter *rate.Limiter
	clock         Clock
	mu            sync.Mutex
	localLimit    rate.Limit
	conns         map[Conn]struct{}
}

// LimitConn wraps the given connection into a bandwidth-limited connection.
func (l *Limiter) LimitConn(conn net.Conn) Conn {
	l.mu.Lock()
	ret := wrapConn(
		conn,
		l.globalLimiter,
		rate.NewLimiter(l.localLimit, chunkSize),
		l.clock,
		l.deleteConn,
	)
	l.conns[ret] = struct{}{}
	l.mu.Unlock()
	return ret
}

// SetGlobalLimit sets the global limit to Limit bytes per second.
// The global limit is a cumulative bandwidth limit for all connections wrapped
// by the Limiter. It takes precedence over the connection's local limit.
//
// Returns ErrInvalidLimit when Limit is negative.
func (l *Limiter) SetGlobalLimit(limit rate.Limit) error {
	if limit < 0 {
		return ErrInvalidLimit
	}

	l.globalLimiter.SetLimitAt(l.clock.Now(), limit)
	return nil
}

// GlobalLimit resturn the current global Limit.
func (l *Limiter) GlobalLimit() rate.Limit {
	return l.globalLimiter.Limit()
}

// SetLocalLimit sets the per-connection Limit (bytes per second)
// for all the connections wrapped by the Limiter. Even if the per-connection
// limit could be set to be greater than the global limit, the latter
// takes precedence.
//
// There's a guarantee that all of the connections shaped by the Limiter
// have a new local limit set when this method finishes execution.
//
// Returns ErrInvalidLimit when Limit is negative.
func (l *Limiter) SetLocalLimit(limit rate.Limit) error {
	if limit < 0 {
		return ErrInvalidLimit
	}

	l.mu.Lock()
	l.localLimit = limit
	for conn := range l.conns {
		conn.SetLimit(limit)
	}
	l.mu.Unlock()

	return nil
}

// LocalLimit returns the current local Limit.
func (l *Limiter) LocalLimit() (ret rate.Limit) {
	l.mu.Lock()
	ret = l.localLimit
	l.mu.Unlock()
	return
}

func (l *Limiter) deleteConn(conn Conn) {
	l.mu.Lock()
	delete(l.conns, conn)
	l.mu.Unlock()
}

type LimiterOption func(*Limiter)

// WithGlobalLimit is a Limiter option that sets the global
// bandwidth limit. See SetGlobalLimit for more information.
func WithGlobalLimit(limit rate.Limit) LimiterOption {
	return func(l *Limiter) {
		l.globalLimiter = rate.NewLimiter(rate.Inf, chunkSize)
	}
}

// WithLocalLimit is a Limiter option that sets the local bandwidth
// limit aka per connection limit. See SetGlobalLimit for more
// information.
func WithLocalLimit(limit rate.Limit) LimiterOption {
	return func(l *Limiter) {
		l.localLimit = limit
	}
}

// WithClock provides the Limiter with a different clock implementation
// than the standard one. Might be useful for mocking purposes.
func WithClock(clock Clock) LimiterOption {
	return func(l *Limiter) {
		l.clock = clock
	}
}

// NewLimiter creates a new Limiter instance using the provided options.
// By default, the Limiter has no limits set, which means it doesn't shape
// traffic and uses a system clock implementation.
func NewLimiter(opts ...LimiterOption) (ret *Limiter) {
	ret = &Limiter{
		globalLimiter: rate.NewLimiter(rate.Inf, chunkSize),
		localLimit:    rate.Inf,
		conns:         make(map[Conn]struct{}),
		clock:         defaultClock,
	}

	for _, opt := range opts {
		opt(ret)
	}
	return
}
