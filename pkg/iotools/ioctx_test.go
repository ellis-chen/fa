package iotools

import (
	"bytes"
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

func TestCopy(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	fIn, err := os.Open("/dev/zero")
	if err != nil {
		log.Fatal(err)
	}
	defer fIn.Close()

	fOut, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		log.Fatal(err)
	}
	defer fOut.Close()

	go func() {
		n, err := Copy(ctx, fOut, fIn)
		log.Println(n, "bytes copied.")
		require.True(t, errors.Is(err, context.DeadlineExceeded))
	}()

	<-time.After(time.Second * 3)

	log.Println("Closing.")
}

func TestCopyWithLimit(t *testing.T) {
	fOut, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		log.Fatal(err)
	}
	defer fOut.Close()

	var ss = []struct {
		name    string
		timeout time.Duration
		expect  int64
	}{
		{name: "100", timeout: time.Millisecond * 100, expect: 1024 * 1},
		{name: "200", timeout: time.Millisecond * 200, expect: 1024 * 2},
		{name: "300", timeout: time.Millisecond * 300, expect: 1024 * 3},
		{name: "400", timeout: time.Millisecond * 400, expect: 1024 * 4},
		{name: "500", timeout: time.Millisecond * 500, expect: 1024 * 5},
	}

	for _, v := range ss {
		v := v
		t.Run(v.name, func(tt *testing.T) {
			r := SleepLimitReader(bytes.NewReader(bytes.Repeat([]byte{0}, 1<<20)), 1<<20, 1<<10, time.Millisecond*100)

			ctx, cancel := context.WithTimeout(context.Background(), v.timeout)
			defer cancel()
			n, err := Copy(ctx, fOut, r)

			t.Logf("Copid %d bytes", n)

			require.ErrorIs(t, err, context.DeadlineExceeded)
			require.EqualValues(t, v.expect, n)
		})
	}
}

var c int32

func contextCancelCase(ctx context.Context, sleep time.Duration) bool {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	deadline, _ := ctx.Deadline()
	logrus.Infof("deadline to now: %v", time.Since(deadline))
	select {
	case <-ctx.Done():
		return true
	default:
		atomic.AddInt32(&c, 1)
		time.Sleep(sleep)

		return false
	}
}

// go test -test.run ^TestContextCancelCase$ -o test
// for i in {1..100}; do ./test -test.run TestContextCancelCase; done;
func TestContextCancelCase(t *testing.T) {
	t.Skip()
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*77)
	defer cancel()
	for !contextCancelCase(ctx, time.Millisecond*33) {

	}

	require.EqualValues(t, 3, c)
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	// interrupt context with SIGTERM (CTRL+C)
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt)
	go func() {
		<-sigs
		cancel()
	}()
	err := CopyDirWithContext(ctx, "a", "b")
	if err != nil {
		log.Println(err)
	}

	<-time.After(time.Second * 30)
	log.Println("exit the copy")
}
