package ctx

import (
	"context"
	"time"
)

type Context interface {
	context.Context
	Duration() time.Duration
}

type logContext struct {
	context.Context
	start time.Time
}

func (l *logContext) Duration() time.Duration {
	return time.Since(l.start)
}

func NewLogContext(ctx context.Context) Context {
	return &logContext{Context: ctx, start: time.Now()}
}
