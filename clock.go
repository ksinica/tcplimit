package tcplimit

import (
	"time"
)

// Clock provides an interface for accessing the monotonic clock.
type Clock interface {
	// Now returns the current local time.
	Now() time.Time
	// Sleep pauses the current goroutine for at least the duration d.
	// A negative or zero duration causes Sleep to return immediately.
	Sleep(d time.Duration)
}

var defaultClock Clock = new(timeClock) // Stateless

type timeClock struct{}

func (c *timeClock) Now() time.Time {
	return time.Now()
}

func (c *timeClock) Sleep(d time.Duration) {
	time.Sleep(d)
}
