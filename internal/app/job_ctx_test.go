package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestJobCtxOuterCancel(t *testing.T) {
	jc := NewJobCtx(time.Second * 5)
	defer jc.Stop()

	type jobState int

	const (
		JobSuccess jobState = iota
		JobFailed
		JobOuterTimeout
		JobInnerTimeout
	)

	type job struct {
		key      string
		timeout  time.Duration
		jf       JobCb
		expected jobState
	}

	jobSet := []job{
		{key: "a", timeout: time.Second * 3, jf: func(ctx context.Context) error {
			time.Sleep(time.Second * 1)

			return nil
		}, expected: JobSuccess},
		{key: "b", timeout: time.Second * 2, jf: func(ctx context.Context) error {
			time.Sleep(time.Second * 4)

			return nil
		}, expected: JobInnerTimeout},
		{key: "c", timeout: time.Second * 10, jf: func(ctx context.Context) error {
			time.Sleep(time.Second * 6)

			return nil
		}, expected: JobOuterTimeout},
		{key: "d", timeout: time.Second * 2, jf: func(ctx context.Context) error {
			time.Sleep(time.Second * 1)

			return errors.New("job err")
		}, expected: JobFailed},
	}

	for _, j := range jobSet {
		t.Run(j.key, func(t *testing.T) {
			outerCtx, cancel := context.WithTimeout(context.TODO(), time.Second*3)
			defer cancel()

			ctx, cl, sc, ec := jc.RegisterJobWithTimeout(outerCtx, j.key, j.timeout, j.jf)

			defer cl()

			var state jobState

			select {
			case <-outerCtx.Done():
				state = JobOuterTimeout
			case <-ctx.Done():
				state = JobInnerTimeout
			case <-sc:
				state = JobSuccess
			case <-ec:
				state = JobFailed
			}

			require.Equal(t, j.expected, state, "mismatch state")
		})
	}
}
