// Package metrics provides a wrapper around a PostgreSQL database connection pool
// that collects metrics for database operations. It implements query timing,
// error tracking, and supports context-based timeouts.
//
// The package is designed to be used as a wrapper around your existing database connection pool
// when you need to collect metrics about database operations in your application.
//
//go:generate mockery
package metrics

import (
	"context"
	"time"

	"github.com/godepo/elephant/internal/pkg/monads"
	"github.com/godepo/elephant/internal/pkg/pgcontext"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Pool interface defines the required database operations that can be measured.
// It extends standard database operations with transaction support.
type Pool interface {
	BeginTx(ctx context.Context, opts pgx.TxOptions) (pgx.Tx, error)
	Begin(ctx context.Context) (pgx.Tx, error)
	Query(ctx context.Context, query string, args ...interface{}) (pgx.Rows, error)
	QueryRow(ctx context.Context, query string, args ...interface{}) pgx.Row
	Exec(ctx context.Context, query string, args ...interface{}) (pgconn.CommandTag, error)
	Transactional(ctx context.Context, fn func(ctx context.Context) error) (out error)
}

// Collector interface defines methods for tracking database operation metrics.
type Collector interface {
	TrackQueryMetrics(ctx context.Context, begin time.Time, err error)
}

// DB represents a metrics-enabled database instance.
type DB struct {
	defaultMetricsCollector Collector
	db                      Pool
}

// New creates a new metrics-enabled database wrapper.
//
// Parameters:
//   - db: The underlying database pool to wrap.
//   - collector: The metrics collector to use for tracking operations.
//
// Returns:
//   - DB: A new metrics-enabled database instance.
func New(db Pool, collector Collector) DB {
	return DB{
		db:                      db,
		defaultMetricsCollector: collector,
	}
}

func (m DB) BeginTx(ctx context.Context, opts pgx.TxOptions) (pgx.Tx, error) {
	return m.db.BeginTx(ctx, opts)
}

func (m DB) Begin(ctx context.Context) (pgx.Tx, error) {
	return m.db.Begin(ctx)
}

// Query executes a query and returns the results with metrics tracking
//
// Features:
//   - Tracks query execution time
//   - Supports query timeouts via context
//   - Records errors through the metrics collector
//
// Example usage:
//
//	ctx = elephant.With(ctx, elephant.WithTimeout(time.Second))
//	       rows, err := db.Query(ctx, "SELECT * FROM users WHERE age > $1", 18)
func (m DB) Query(ctx context.Context, query string, args ...interface{}) (pgx.Rows, error) {
	begin := time.Now()
	rows, err := m.db.Query(ctx, query, args...)
	if err != nil {
		m.defaultMetricsCollector.TrackQueryMetrics(ctx, begin, err)
		return nil, err
	}
	cancel := monads.EmptyOf[context.CancelFunc]()
	if timeout, ok := pgcontext.QueryTimeoutFrom(ctx); ok {
		cancelCtx, cancelFunc := context.WithTimeout(ctx, timeout)
		ctx, cancel = cancelCtx, monads.OptionalOf(cancelFunc)
	}
	return newDecoratedRows(ctx, rows, cancel, begin, m.defaultMetricsCollector), nil
}

// QueryRow executes a query that returns a single row with metrics tracking
//
// Features:
//   - Tracks query execution time
//   - Supports query timeouts via context
//   - Records metrics through the collector
func (m DB) QueryRow(ctx context.Context, query string, args ...interface{}) pgx.Row {
	row := decoratedMetricRow{
		ctx:       ctx,
		begin:     time.Now(),
		row:       m.db.QueryRow(ctx, query, args...),
		collector: m.defaultMetricsCollector,
	}
	timeout, ok := pgcontext.QueryTimeoutFrom(ctx)
	if ok {
		cancelCtx, cancel := context.WithTimeout(ctx, timeout)
		row.ctx = cancelCtx
		row.cancel = monads.OptionalOf(cancel)
	}
	return row
}

// Exec executes a command (like INSERT, UPDATE, DELETE) with metrics tracking
//
// Features:
//   - Tracks execution time
//   - Records success/failure metrics
//   - Returns affected row count via CommandTag
func (m DB) Exec(ctx context.Context, query string, args ...interface{}) (pgconn.CommandTag, error) {
	begin := time.Now()
	tag, err := m.db.Exec(ctx, query, args...)
	if err != nil {
		m.defaultMetricsCollector.TrackQueryMetrics(ctx, begin, err)
		return tag, err
	}
	m.defaultMetricsCollector.TrackQueryMetrics(ctx, begin, err)
	return tag, nil
}

// Transactional executes the provided function within a database transaction
//
// Features:
//   - Delegates to underlying Pool's transaction handling
//   - Maintains metrics collection within transaction
func (m DB) Transactional(ctx context.Context, fn func(ctx context.Context) error) (out error) {
	return m.db.Transactional(ctx, fn)
}
