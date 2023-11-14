package logger

import (
	"fmt"
	"time"

	"github.com/ellis-chen/fa/internal/ctx"
)

func Timeit(ctx ctx.Context, format string, args ...interface{}) {
	used := ctx.Duration()
	cmd := fmt.Sprintf(format, args...)
	t := time.Now()
	ts := t.Format("2006.01.02 15:04:05.000000")
	cmd += fmt.Sprintf(" <%.6f>", used.Seconds())
	cmd = fmt.Sprintf("%s %s", ts, cmd)
	Debugf(ctx, cmd)
}
