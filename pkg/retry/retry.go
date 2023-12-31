package retry

import (
	"context"
	"time"

	"github.com/jpillora/backoff"
	"github.com/pkg/errors"
)

// Func returns true if the condition is satisfied, or an error if the loop
// should be aborted.
type Func func(context.Context) (bool, error)

// IsRetryableFunc is the signature for functions that return true if we should
// retry an error
type IsRetryableFunc func(error) bool

// IsAlwaysRetryable instructs WaitWithRetries to retry until time expires.
func IsAlwaysRetryable(error) bool {
	return true
}

// IsNeverRetryable instructs WaitWithRetries not to retry.
func IsNeverRetryable(error) bool {
	return false
}

// IsRetryOnContext will retry if err is not context.Canceled
func IsRetryOnContext(err error) bool {
	return !errors.Is(err, context.Canceled)
}

// Wait calls WaitWithBackoff with default backoff parameters. The defaults are
// handled by the "github.com/jpillora/backoff" and are:
//
//	min = 100 * time.Millisecond
//	max = 10 * time.Second
//	factor = 2
//	jitter = false
func Wait(ctx context.Context, f Func) error {
	return WaitWithBackoff(ctx, backoff.Backoff{}, f)
}

// WaitWithBackoff calls a function until it returns true, an error, or until
// the context is done.
func WaitWithBackoff(ctx context.Context, b backoff.Backoff, f Func) error {
	return WaitWithBackoffAndRetries(ctx, b, 0, IsNeverRetryable, f)
}

// WaitWithRetries will invoke a function `f` until it returns true or the
// context `ctx` is done. If `f` returns an error, WaitWithRetries will tolerate
// up to `numRetries` errors.
func WaitWithRetries(ctx context.Context, numRetries int, r IsRetryableFunc, f Func) error {
	return WaitWithBackoffAndRetries(ctx, backoff.Backoff{}, numRetries, r, f)
}

// WaitWithBackoffAndRetries will invoke a function `f` until it returns true or the
// context `ctx` is done. If `f` returns an error, WaitWithBackoffWith retries will tolerate
// up to `numRetries` errors. If returned error is not retriable according to `r`, then
// it will bait out immediately. The wait time between retries will be decided by backoff
// parameters `b`.
func WaitWithBackoffAndRetries(ctx context.Context, b backoff.Backoff, numRetries int, r IsRetryableFunc, f Func) error {
	if numRetries < 0 {
		return errors.New("numRetries must be non-negative")
	}

	t := time.NewTimer(0)
	<-t.C
	retries := 0
	for {
		ok, err := f(ctx)
		switch {
		case err != nil:
			if !r(err) || retries >= numRetries {
				return err
			}
			retries++
		case ok:
			return nil
		}
		sleep := b.Duration()
		if deadline, ok := ctx.Deadline(); ok {
			ctxSleep := time.Until(deadline)
			sleep = minDuration(sleep, ctxSleep)
		}
		t.Reset(sleep)
		select {
		case <-ctx.Done():
			return errors.Wrap(ctx.Err(), "Context done while retrying")
		case <-t.C:
		}
	}
}

func minDuration(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}
