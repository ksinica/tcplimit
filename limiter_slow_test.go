//go:build slow
// +build slow

package tcplimit_test

import (
	"errors"
	"io"
	"testing"
	"time"

	"github.com/ksinica/tcplimit"
	"github.com/ksinica/tcplimit/internal/pkg/mock"
	"golang.org/x/time/rate"
)

func performMeasurement(t *testing.T, limiter *tcplimit.Limiter, d time.Duration) float64 {
	conn := mock.NewNoopConn()

	limitedConn := limiter.LimitConn(mock.NewNoopConn())
	go func() {
		time.Sleep(d)
		limitedConn.Close()
		conn.Close()
	}()

	now := time.Now()

	c := make(chan int64)
	go func() {
		n, err := io.Copy(limitedConn, conn)
		if err != nil && !errors.Is(err, io.EOF) {
			t.Error(err)
		}
		c <- n
	}()

	n, err := io.Copy(conn, limitedConn)
	if err != nil && !errors.Is(err, io.EOF) {
		t.Error(err)
	}
	n += <-c

	return float64(n) / time.Since(now).Seconds()
}

func calculateDeviationInPercent(a, b float64) float64 {
	return 100.0 - (min(a, b)/max(a, b))*100
}

func TestLimiterConstraints(t *testing.T) {
	for _, test := range []struct {
		name                string
		duration            time.Duration
		globalLimit         rate.Limit
		localLimit          rate.Limit
		connectionCount     int
		expectedBandwidth   float64
		acceptableDeviation float64
	}{
		{
			name:                "bandwidth should not deviate more than 5 percent from global",
			duration:            time.Second * 30,
			globalLimit:         rate.Limit(333 * 1024),
			localLimit:          rate.Inf,
			connectionCount:     1,
			expectedBandwidth:   333 * 1024,
			acceptableDeviation: 5,
		},
		{
			name:                "bandwidth should not deviate more than 5 percent from local",
			duration:            time.Second * 30,
			globalLimit:         rate.Limit(10 * 1024),
			localLimit:          rate.Limit(5 * 1024),
			connectionCount:     1,
			expectedBandwidth:   5 * 1024,
			acceptableDeviation: 5,
		},
		{
			name:                "bandwidth should not deviate more than 5 percent from global (10 connections)",
			duration:            time.Second * 30,
			globalLimit:         rate.Limit(333 * 1024),
			localLimit:          rate.Inf,
			connectionCount:     10,
			expectedBandwidth:   333 * 1024,
			acceptableDeviation: 5,
		},
		{
			name:                "bandwidth should not deviate more than 5 percent from local (10 connections)",
			duration:            time.Second * 30,
			globalLimit:         rate.Limit(60 * 1024),
			localLimit:          rate.Limit(5 * 1024),
			connectionCount:     10,
			expectedBandwidth:   50 * 1024, // 5kB * 10
			acceptableDeviation: 5,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			limiter := tcplimit.NewLimiter()
			limiter.SetGlobalLimit(test.globalLimit)
			limiter.SetLocalLimit(test.localLimit)

			c := make(chan float64, test.connectionCount)

			for i := 0; i < test.connectionCount; i++ {
				go func() {
					c <- performMeasurement(t, limiter, test.duration)
				}()
			}

			var bandwidth float64
			for i := 0; i < test.connectionCount; i++ {
				bandwidth += <-c
			}

			gotDeviation := calculateDeviationInPercent(test.expectedBandwidth, bandwidth)
			if gotDeviation > test.acceptableDeviation {
				t.Errorf(
					"expected deviation to be less than %f, got %f",
					test.acceptableDeviation,
					gotDeviation,
				)
			}
		})
	}
}
