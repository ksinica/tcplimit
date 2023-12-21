package tcplimit

import (
	"bytes"
	"errors"
	"io"
	"reflect"
	"testing"
	"time"

	"github.com/ksinica/tcplimit/internal/pkg/mock"
	"golang.org/x/time/rate"
)

func TestForEachChunk(t *testing.T) {
	for _, test := range []struct {
		name     string
		input    []int
		size     int
		expected [][]int
	}{
		{
			name:  "invalid chunk size",
			input: []int{1, 2, 3},
			size:  -1,
		},
		{
			name:  "zero chunk size",
			input: []int{1, 2, 3},
			size:  0,
		},
		{
			name:     "individual elements",
			input:    []int{1, 2, 3},
			size:     1,
			expected: [][]int{{1}, {2}, {3}},
		},
		{
			name:     "even and odd sized elements",
			input:    []int{1, 2, 3},
			size:     2,
			expected: [][]int{{1, 2}, {3}},
		},
		{
			name:     "one element #1",
			input:    []int{1, 2, 3},
			size:     3,
			expected: [][]int{{1, 2, 3}},
		},
		{
			name:     "one element #2",
			input:    []int{1, 2, 3},
			size:     4,
			expected: [][]int{{1, 2, 3}},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			var got [][]int
			forEachChunk(test.input, test.size, func(p []int) bool {
				got = append(got, p)
				return true
			})
			if !reflect.DeepEqual(test.expected, got) {
				t.Errorf("Expected %v, got %v", test.expected, got)
			}
		})
	}
}

func TestConnErrUnfulfillableLocalReservation(t *testing.T) {
	var input = makeByteSliceWithTestData(chunkSize * 10)

	conn := wrapConn(
		mock.NewNoopConn(),
		rate.NewLimiter(rate.Inf, 0),
		rate.NewLimiter(rate.Limit(0), chunkSize),
		defaultClock,
		func(Conn) {},
	)
	defer conn.Close()

	t.Run("write", func(t *testing.T) {
		_, err := conn.Write(input)
		if !errors.Is(err, ErrUnfulfillableReservation) {
			t.Errorf("expected %s, got %s", ErrUnfulfillableReservation, err)
		}
	})
	t.Run("read", func(t *testing.T) {
		var p [chunkSize]byte
		_, err := conn.Read(p[:])
		if !errors.Is(err, ErrUnfulfillableReservation) {
			t.Errorf("expected %s, got %s", ErrUnfulfillableReservation, err)
		}
	})
}

func TestConnReadWriteConsistency(t *testing.T) {
	now := time.Now()
	clock := &mock.Clock{
		OnNow: func() (ret time.Time) {
			ret = now
			now = now.Add(time.Second)
			return
		},
	}

	t.Run("write", func(t *testing.T) {
		var expected = makeByteSliceWithTestData(chunkSize * 10)

		conn := wrapConn(
			mock.NewBufferConn(expected),
			rate.NewLimiter(rate.Inf, 0),
			rate.NewLimiter(rate.Limit(3), chunkSize),
			clock,
			func(Conn) {},
		)
		defer conn.Close()

		var got bytes.Buffer
		_, err := io.Copy(&got, conn)
		if err != nil {
			t.Error("unexpected error:", err)
		}

		if !bytes.Equal(expected, got.Bytes()) {
			t.Error("unexpected write result")
		}
	})

	t.Run("read", func(t *testing.T) {
		var expected = makeByteSliceWithTestData(chunkSize * 10)

		got := mock.NewBufferConn(nil)
		conn := wrapConn(
			got,
			rate.NewLimiter(rate.Inf, 0),
			rate.NewLimiter(rate.Limit(3), chunkSize),
			clock,
			func(Conn) {},
		)
		defer conn.Close()

		_, err := io.Copy(conn, bytes.NewBuffer(expected))
		if err != nil {
			t.Error("unexpected error:", err)
		}

		if !bytes.Equal(expected, got.(mock.BufferReturner).Bytes()) {
			t.Error("unexpected read result")
		}
	})
}

func TestConnInvalidLimit(t *testing.T) {
	conn := wrapConn(
		mock.NewNoopConn(),
		rate.NewLimiter(rate.Inf, 0),
		rate.NewLimiter(rate.Inf, 0),
		defaultClock,
		func(Conn) {},
	)
	defer conn.Close()

	err := conn.SetLimit(rate.Limit(-1))
	if !errors.Is(err, ErrInvalidLimit) {
		t.Errorf("expected %s, got %s", ErrInvalidLimit, err)
	}
}

func makeByteSliceWithTestData(n int) (ret []byte) {
	ret = make([]byte, n)
	for i := range ret {
		ret[i] = byte(i % 0xff)
	}
	return
}
