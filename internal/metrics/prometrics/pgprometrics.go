package prometrics

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/godepo/elephant/internal/metrics"
	"github.com/godepo/elephant/internal/pkg/pgcontext"
	"github.com/prometheus/client_golang/prometheus"
)

func defaultInterceptor(ctx context.Context, err error) int {
	if err != nil {
		return metrics.InterceptAsFailure
	}
	return metrics.InterceptAsSuccess
}

type MetricConstructor func(label string) (
	qps []prometheus.Counter,
	latency prometheus.Histogram,
	latenciesByResult []prometheus.Histogram,
)

type Builder interface {
	QueryPerSecond(label string, metrics ...prometheus.Counter) Builder
	Latency(label string, metric prometheus.Histogram) Builder
	LatencyByResult(label string, metrics ...prometheus.Histogram) Builder
	Constructor(constructor MetricConstructor) Builder
	CanDynamic() Builder
	Build() (*Collector, error)
}

type builder struct {
	qps               map[string][]prometheus.Counter
	latencies         map[string]prometheus.Histogram
	latenciesByResult map[string][]prometheus.Histogram
	coll              prometheus.Collector
	constructor       MetricConstructor
	hasConstructor    bool
	canDynamic        bool
}

func (b builder) Constructor(constructor MetricConstructor) Builder {
	out := b.clone()
	out.constructor = constructor
	out.hasConstructor = true
	return out
}

func (b builder) CanDynamic() Builder {
	out := b.clone()
	b.canDynamic = true
	return out
}

func (b builder) clone() builder {
	return b
}

func (b builder) QueryPerSecond(label string, metrics ...prometheus.Counter) Builder {
	out := b.clone()
	out.qps[label] = metrics
	return out
}

func (b builder) Latency(label string, metric prometheus.Histogram) Builder {
	out := b.clone()
	out.latencies[label] = metric
	return out
}

func (b builder) LatencyByResult(label string, metrics ...prometheus.Histogram) Builder {
	out := b.clone()
	out.latenciesByResult[label] = metrics
	return out
}

func (b builder) Build() (*Collector, error) {
	if len(b.qps) != len(b.latencies) || len(b.latenciesByResult) != len(b.qps) {
		return nil, fmt.Errorf("qps and latencies and latenciesByResult are not the same length")
	}

	metricsIx := make(map[string]*collectingGroup)

	for k, v := range b.qps {
		metricsIx[k] = &collectingGroup{
			qps: v,
		}
	}

	for k, v := range b.latencies {
		grp, ok := metricsIx[k]
		if !ok {
			return nil, fmt.Errorf("incorrect metrics specify cant' find qps by key '%s'", k)
		}
		grp.latency = v
	}

	for k, v := range b.latenciesByResult {
		grp, ok := metricsIx[k]
		if !ok {
			return nil, fmt.Errorf("incorrect metrics specify cant' find qps and latency by key '%s'", k)
		}
		grp.latenciesByResult = v
	}

	clt := &Collector{
		interceptor: defaultInterceptor,
		metrics:     metricsIx,
	}
	clt.tracker = clt.trackStaticQuery
	if b.canDynamic && !b.hasConstructor {
		return nil, fmt.Errorf("constructor requires for dynamic metrics collectors")
	}
	if b.canDynamic {
		clt.tracker = clt.trackDynamicQuery
		clt.constructor = b.constructor
		clt.lock = &sync.RWMutex{}
	}
	return clt, nil
}

func New() Builder {
	return builder{
		qps:               make(map[string][]prometheus.Counter),
		latencies:         make(map[string]prometheus.Histogram),
		latenciesByResult: make(map[string][]prometheus.Histogram),
	}
}

type collectingGroup struct {
	qps               []prometheus.Counter
	latency           prometheus.Histogram
	latenciesByResult []prometheus.Histogram
}

type Collector struct {
	interceptor metrics.Interceptor
	metrics     map[string]*collectingGroup
	tracker     func(ctx context.Context, begin time.Time, err error)
	constructor MetricConstructor
	lock        *sync.RWMutex
}

func (clt *Collector) TrackQuery(ctx context.Context, begin time.Time, err error) {
	clt.tracker(ctx, begin, err)
}

func (clt *Collector) track(ctx context.Context, begin time.Time, err error, group *collectingGroup) {
	interceptor := clt.interceptor
	ix := interceptor(ctx, err)

	if len(group.qps) > ix {
		group.qps[ix].Inc()
	}

	since := float64(time.Since(begin).Milliseconds())

	group.latency.Observe(since)

	if len(group.latenciesByResult) > ix {
		group.latenciesByResult[ix].Observe(since)
	}
}

func (clt *Collector) trackStaticQuery(ctx context.Context, begin time.Time, err error) {
	label, ok := pgcontext.MetricsLabelFrom(ctx)
	if !ok {
		return
	}

	grp, ok := clt.metrics[label]
	if !ok {
		return
	}

	clt.track(ctx, begin, err, grp)
}

func (clt *Collector) trackDynamicQuery(ctx context.Context, begin time.Time, err error) {
	if clt.trackExists(ctx, begin, err) {
		return
	}
	clt.lock.Lock()
	defer clt.lock.Unlock()

	label, _ := pgcontext.MetricsLabelFrom(ctx)

	qps, latency, latenciesByResult := clt.constructor(label)
	grp := &collectingGroup{
		qps:               qps,
		latency:           latency,
		latenciesByResult: latenciesByResult,
	}
	clt.metrics[label] = grp
	go clt.track(ctx, begin, err, clt.metrics[label])
}

func (clt *Collector) trackExists(ctx context.Context, begin time.Time, err error) bool {
	clt.lock.RLock()
	defer clt.lock.RUnlock()

	label, ok := pgcontext.MetricsLabelFrom(ctx)
	if !ok {
		return true
	}
	grp, ok := clt.metrics[label]
	if !ok {
		return false
	}

	go clt.track(ctx, begin, err, grp)
	return true
}
