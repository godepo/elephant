package metrics

import (
	"context"
	"time"
)

const (
	InterceptAsFailure = 0
	InterceptAsSuccess = 1
)

type Interceptor func(ctx context.Context, err error) int

type Collector interface {
	TrackQuery(ctx context.Context, begin time.Time, query string)
}
