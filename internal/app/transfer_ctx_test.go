package app

import (
	"bytes"
	"context"
	"log"
	"os"
	"testing"
	"time"

	"github.com/ellis-chen/fa/pkg/iotools"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

func TestAppTransferJobCtx_GetJobCtx(t *testing.T) {
	atjc := newAppTransferJobCtx(time.Millisecond * 500)
	defer atjc.Stop()
	var start = time.Now()

	ctx, cancel, err := atjc.RegisterJob("app1", func(_ context.Context, _ interface{}) {
		time.Sleep(100 * time.Millisecond)
		t.Logf("It cost %.2f sec to finish the job", float64(time.Since(start).Milliseconds())/1e3)
	})
	if err != nil {
		t.Error(err)
	}

	defer cancel()

	ctx2, _, _ := atjc.RegisterJob("app1", func(_ context.Context, _ interface{}) {
		time.Sleep(100 * time.Millisecond)
		t.Logf("It cost %.2f sec to finish the job", float64(time.Since(start).Milliseconds())/1e3)
	})

	ctx3, _ := atjc.GetJobCtx("app1")

	require.Equal(t, ctx, ctx2)
	require.Equal(t, ctx, ctx3)

	<-ctx.Done()
	t.Logf("ctx done cost %.2f sec", float64(time.Since(start).Milliseconds())/1e3)

	<-time.After(time.Second * 1)
	t.Logf("exit the test code")
}

func TestAppTransferJobCtx_Batch(t *testing.T) {
	if os.Getenv("TEST_IGNORE") == "true" {
		t.Skip()
	}
	atjc := newAppTransferJobCtx(time.Millisecond * 250)
	defer atjc.Stop()

	var ss = []struct {
		name  string
		sleep time.Duration
		jf    JobFunc
		err   error
	}{
		{name: "app1", sleep: time.Millisecond * 100, err: context.Canceled},
		{name: "app2", sleep: time.Millisecond * 200, err: context.Canceled},
		{name: "app3", sleep: time.Millisecond * 300, err: context.DeadlineExceeded},
		{name: "app4", sleep: time.Millisecond * 400, err: context.DeadlineExceeded},
		{name: "app5", sleep: time.Millisecond * 500, err: context.DeadlineExceeded},
	}

	for _, v := range ss {
		v := v
		t.Run(v.name, func(t *testing.T) {
			var start = time.Now()

			ctx, cancel, err := atjc.RegisterJob(v.name, func(_ context.Context, data interface{}) {
				time.Sleep(v.sleep)
				logrus.Infof("app [ %v ] cost %.2f sec to finish the job", data, float64(time.Since(start).Milliseconds())/1e3)
			})
			require.NoError(t, err)

			defer cancel()
			<-ctx.Done()
			logrus.Infof("app [ %v ] ctx done cost %.2f sec", v.name, float64(time.Since(start).Milliseconds())/1e3)

			require.Equal(t, v.err, ctx.Err())
		})
	}

	<-time.After(time.Second)
}

func TestAppTransferJobCtx_BatchWithCopy(t *testing.T) {
	if os.Getenv("TEST_IGNORE") == "true" {
		t.Skip()
	}
	atjc := newAppTransferJobCtx(time.Millisecond * 300)
	defer atjc.Stop()

	fOut, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		log.Fatal(err)
	}
	//defer fOut.Close()

	var ss = []struct {
		name       string
		bufSize    int
		unitSize   int64
		timeout    time.Duration
		jf         JobFunc
		err        error
		copyErr    error
		expectRead int
	}{
		{name: "app1", bufSize: 1 << 10, unitSize: 1 << 10, timeout: time.Millisecond * 200, copyErr: nil, err: context.Canceled, expectRead: 1 << 10},
		{name: "app2", bufSize: 2 << 10, unitSize: 1 << 10, timeout: time.Millisecond * 200, copyErr: nil, err: context.Canceled, expectRead: 2 << 10},
		{name: "app3", bufSize: 4 << 10, unitSize: 1 << 10, timeout: time.Millisecond * 200, copyErr: context.DeadlineExceeded, err: context.DeadlineExceeded, expectRead: 4 << 10},
		{name: "app4", bufSize: 8 << 10, unitSize: 1 << 10, timeout: time.Millisecond * 200, copyErr: context.DeadlineExceeded, err: context.DeadlineExceeded, expectRead: 4 << 10},
		{name: "app5", bufSize: 8 << 10, unitSize: 2 << 10, timeout: time.Millisecond * 200, copyErr: context.DeadlineExceeded, err: context.DeadlineExceeded, expectRead: 8 << 10},
		{name: "app6", bufSize: 8 << 10, unitSize: 1 << 10, timeout: time.Millisecond * 500, copyErr: nil, err: context.Canceled, expectRead: 8 << 10},
	}

	for _, v := range ss {
		v := v
		t.Run(v.name, func(t *testing.T) {
			var start = time.Now()

			ctx, _, err := atjc.RegisterJobWithTimeout(v.name, v.timeout, func(ctx context.Context, _ interface{}) {
				fixReader := bytes.NewReader(bytes.Repeat([]byte{0}, v.bufSize))
				r := iotools.SleepLimitReader(fixReader, int64(v.bufSize), v.unitSize, time.Millisecond*50)

				n, err := iotools.Copy(ctx, fOut, r)

				if v.copyErr != nil {
					require.Error(t, err)
				} else {
					require.NoError(t, err)
				}
				require.ErrorIs(t, err, v.copyErr)
				require.EqualValues(t, v.expectRead, n)
			})
			require.NoError(t, err)

			<-ctx.Done()
			logrus.Infof("app [ %v ] ctx done cost %.2f sec", v.name, float64(time.Since(start).Milliseconds())/1e3)
			// require.Equal(t, v.err, ctx.Err())
		})
	}

	<-time.After(time.Second)
}

func TestAppTransferJobCtx_BatchWithCopyAndCancel(t *testing.T) {
	if os.Getenv("TEST_IGNORE") == "true" {
		t.Skip()
	}
	atjc := newAppTransferJobCtx(time.Millisecond * 300)
	defer atjc.Stop()

	fOut, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		log.Fatal(err)
	}
	//defer fOut.Close()

	var ss = []struct {
		name          string
		bufSize       int
		unitSize      int64
		timeout       time.Duration
		cancelTimeout time.Duration
		jf            JobFunc
		err           error
		copyErr       error
		expectRead    int64
	}{
		{name: "app1", bufSize: 1 << 10, unitSize: 1 << 10, timeout: time.Millisecond * 200, cancelTimeout: time.Millisecond * 100, copyErr: nil, err: context.Canceled, expectRead: 1 << 10},
		{name: "app2", bufSize: 3 << 10, unitSize: 1 << 10, timeout: time.Millisecond * 200, cancelTimeout: time.Millisecond * 100, copyErr: context.Canceled, err: context.Canceled, expectRead: 2 << 10},
		{name: "app3", bufSize: 4 << 10, unitSize: 1 << 10, timeout: time.Millisecond * 200, cancelTimeout: time.Millisecond * 100, copyErr: context.Canceled, err: context.Canceled, expectRead: 2 << 10},
		{name: "app4", bufSize: 8 << 10, unitSize: 1 << 10, timeout: time.Millisecond * 200, cancelTimeout: time.Millisecond * 100, copyErr: context.Canceled, err: context.Canceled, expectRead: 2 << 10},
		{name: "app5", bufSize: 8 << 10, unitSize: 2 << 10, timeout: time.Millisecond * 200, cancelTimeout: time.Millisecond * 100, copyErr: context.Canceled, err: context.Canceled, expectRead: 4 << 10},
		{name: "app6", bufSize: 8 << 10, unitSize: 1 << 10, timeout: time.Millisecond * 500, cancelTimeout: time.Millisecond * 200, copyErr: context.Canceled, err: context.Canceled, expectRead: 4 << 10},
	}

	for _, v := range ss {
		v := v
		t.Run(v.name, func(t *testing.T) {
			var start = time.Now()

			ctx, cancel, err := atjc.RegisterJobWithTimeout(v.name, v.timeout, func(ctx context.Context, _ interface{}) {
				r := iotools.SleepLimitReader(bytes.NewReader(bytes.Repeat([]byte{0}, v.bufSize)), int64(v.bufSize), v.unitSize, time.Millisecond*50)

				n, err := iotools.Copy(ctx, fOut, r)

				if v.copyErr != nil {
					require.Error(t, err)
				} else {
					require.NoError(t, err)
				}
				require.GreaterOrEqual(t, n, v.expectRead)
			})
			require.NoError(t, err)

			time.AfterFunc(v.cancelTimeout, cancel)

			<-ctx.Done()
			logrus.Infof("app [ %v ] ctx done cost %.2f sec", v.name, float64(time.Since(start).Milliseconds())/1e3)
			require.Equal(t, v.err, ctx.Err())
		})
	}

	<-time.After(time.Second)
}
