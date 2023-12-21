package mock

import "time"

type Clock struct {
	OnNow   func() time.Time
	OnSleep func(time.Duration)
}

func (c *Clock) Now() (ret time.Time) {
	if c.OnNow != nil {
		ret = c.OnNow()
	}
	return
}

func (c *Clock) Sleep(d time.Duration) {
	if c.OnSleep != nil {
		c.OnSleep(d)
	}
}
