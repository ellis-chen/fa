package retry

import (
	"math/rand"
	"testing"
	"time"

	"github.com/jpillora/backoff"
)

func TestBackoff(t *testing.T) {
	rand.Seed(time.Now().UnixMilli())
	b := &backoff.Backoff{
		//These are the defaults
		Min:    200 * time.Millisecond,
		Max:    10 * time.Second,
		Factor: 2,
		Jitter: true,
	}

	t.Logf("%s\n", b.Duration())
	t.Logf("%s\n", b.Duration())
	t.Logf("%s\n", b.Duration())
	t.Logf("%s\n", b.Duration())
	t.Logf("%s\n", b.Duration())
	t.Logf("%s\n", b.Duration())
	t.Logf("%s\n", b.Duration())

	t.Logf("Reset!\n")
	b.Reset()

	t.Logf("%s\n", b.Duration())
}
