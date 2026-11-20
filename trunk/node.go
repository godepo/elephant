package trunk

import (
	"context"

	"github.com/godepo/elephant"
	"github.com/godepo/elephant/internal/regular"
	"github.com/godepo/elephant/metrics"
	"github.com/godepo/elephant/poolstats"
	"github.com/jackc/pgx/v5/pgxpool"
)

func newNode(ctx context.Context, dsn string, settings *Settings, rules ...roleRule) (*DB, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, err
	}

	opts := roleRules{
		metricsCollectorProvider: metrics.Collector,
	}

	for _, rule := range rules {
		rule(&opts)
	}

	opts = opts.init()

	node := regular.New(pool)

	db := DB{
		Instance: node,
		closers:  []dbCloser{pool.Close},
	}

	if settings.registry != nil {
		db.informer = poolstats.InformerProviderFrom(pool)

		clt, err := opts.metricsCollectorProvider().
			Latency(func(labels ...string) (elephant.Histogram, error) {
				return settings.latencies.WithLabelValues(opts.Values(labels)...), nil
			}).
			QueryPerSecond(func(labels ...string) (elephant.Counter, error) {
				return settings.qps.GetMetricWithLabelValues(opts.Values(labels)...)
			}).Build()
		if err != nil {
			return nil, err
		}

		settings.poolStatsOptions = append(settings.poolStatsOptions,
			poolstats.WithInformerProvider(
				poolstats.InformerProviderFrom(pool),
				opts.StatsLabels()...,
			),
		)

		db.Instance = metrics.New(node, clt)
	}

	return &db, nil
}

func (db *DB) Close() {
	for _, closer := range db.closers {
		closer()
	}
}
