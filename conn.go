package tcplimit

import (
	"errors"
	"net"

	"golang.org/x/time/rate"
)

const (
	chunkSize = 1024 // 1kB for now, a trade-off between the number of I/O ops and shaping accuracy.
)

var (
	ErrInvalidLimit = errors.New("invalid limit")
	// ErrUnfulfillableReservation means that the operation cannot fulfill
	// the internal bandwidth reservation.
	ErrUnfulfillableReservation = errors.New("unfulfillable reservation")
)

// Conn is a generic stream-oriented network connection, enriched
// by the ability to set bandwidth limits.
//
// Multiple goroutines may invoke methods on a Conn simultaneously.
type Conn interface {
	net.Conn

	// SetLimitAt sets a new Limit for the limiter. To disable limiting,
	// the Limit can be set to rate.Inf. A zero Limit allows no operations,
	// causing them to fail with ErrUnfulfillableReservation.
	//
	// Returns ErrInvalidLimit when Limit is negative.
	SetLimit(limit rate.Limit) error
	// Limit returns the current Limit.
	Limit() rate.Limit
}

type conn struct {
	net.Conn

	// globalLimiter is a limiter shared between other connections.
	globalLimiter *rate.Limiter
	// localLimiter is a private limiter owned by the connection.
	localLimiter *rate.Limiter
	clock        Clock
	close        func(Conn)
}

func (c *conn) do(p []byte, f func([]byte) (int, error)) (n int, err error) {
	// The bulk of limiter logic resides here. We try to acquire reservations
	// both from the golbal limiter first, then the second. Then take
	// the greater wait time to fulfill one of the reservations.
	now := c.clock.Now()

	globalReservation := c.globalLimiter.ReserveN(now, len(p))
	if !globalReservation.OK() {
		return 0, ErrUnfulfillableReservation
	}

	localReservation := c.localLimiter.ReserveN(now, len(p))
	if !localReservation.OK() {
		globalReservation.CancelAt(now)
		return 0, ErrUnfulfillableReservation
	}

	c.clock.Sleep(
		max(
			globalReservation.DelayFrom(now),
			localReservation.DelayFrom(now),
		),
	)

	n, err = f(p)
	return
}

func (c *conn) Read(p []byte) (int, error) {
	// We are limiting read operation to be capped
	// to the chunk size (== max allowed burst).
	return c.do(
		p[:min(chunkSize, len(p))],
		c.Conn.Read,
	)
}

func (c *conn) Write(p []byte) (n int, err error) {
	// Write is expected to write the whole buffer,
	// so we partition it by chunks (<= limiter's max allowed burst).
	forEachChunk(p, chunkSize, func(p []byte) bool {
		var nn int
		nn, err = c.do(p, c.Conn.Write)
		n += nn
		return err == nil
	})
	return
}

func (c *conn) SetLimit(limit rate.Limit) error {
	if limit < 0 {
		return ErrInvalidLimit
	}

	c.localLimiter.SetLimitAt(c.clock.Now(), limit)
	return nil
}

func (c *conn) Limit() rate.Limit {
	return c.localLimiter.Limit()
}

func (c *conn) Close() error {
	if c.close != nil {
		c.close(c)
	}
	return c.Conn.Close()
}

// forEachChunk divides the slice into a slice of subslices
// capped to the size of elements.
func forEachChunk[T any](p []T, size int, f func(p []T) bool) {
	if size < 1 {
		return
	}

	var j int
	for i := 0; i < len(p); i += size {
		j += size
		if j > len(p) {
			j = len(p)
		}

		if !f(p[i:j]) {
			return
		}
	}
}

// wrapConn wraps net.Conn into bandwidth-limitable implementation.
// Conn's will be unlimited, but can be set implicitly using SetLimit.
func wrapConn(nc net.Conn, global, local *rate.Limiter, clock Clock,
	close func(Conn)) Conn {
	return &conn{
		Conn:          nc,
		globalLimiter: global,
		localLimiter:  local,
		clock:         clock,
		close:         close,
	}
}
