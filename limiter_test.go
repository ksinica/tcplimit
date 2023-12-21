package tcplimit_test

import (
	"errors"
	"testing"

	"github.com/ksinica/tcplimit"
	"github.com/ksinica/tcplimit/internal/pkg/mock"
	"golang.org/x/time/rate"
)

func TestLimiterLocalLimitPropagation(t *testing.T) {
	limiter := tcplimit.NewLimiter()
	limiter.SetLocalLimit(rate.Limit(123))

	var conns []tcplimit.Conn
	for i := 0; i < 10; i++ {
		conns = append(conns, limiter.LimitConn(mock.NewNoopConn()))
	}

	limiter.SetLocalLimit(rate.Limit(321))

	for _, conn := range conns {
		if conn.Limit() != rate.Limit(321) {
			t.Fail()
		}
	}
}

func TestLimiterInvalidGlobalLimit(t *testing.T) {
	limiter := tcplimit.NewLimiter()

	err := limiter.SetGlobalLimit(rate.Limit(-1))
	if !errors.Is(err, tcplimit.ErrInvalidLimit) {
		t.Errorf("expected %s, got %s", tcplimit.ErrInvalidLimit, err)
	}
}

func TestLimiterInvalidLocalLimit(t *testing.T) {
	limiter := tcplimit.NewLimiter()

	err := limiter.SetLocalLimit(rate.Limit(-1))
	if !errors.Is(err, tcplimit.ErrInvalidLimit) {
		t.Errorf("expected %s, got %s", tcplimit.ErrInvalidLimit, err)
	}
}
