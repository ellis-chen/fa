package logger

import (
	"context"
	"testing"
	"time"

	"github.com/ellis-chen/fa/internal/ctx"
	"github.com/sirupsen/logrus"
)

func TestTimeit(t *testing.T) {
	SetLevel(logrus.DebugLevel)
	ctx := ctx.NewLogContext(context.TODO())
	defer func() {
		Timeit(ctx, "hello-world")
	}()

	time.Sleep(time.Second * 2)
}
