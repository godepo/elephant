package metrics

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/godepo/elephant/internal/pkg/pgcontext"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type batchStub struct {
	pgx.BatchResults
	execErr, queryErr, closeErr error
	row                         pgx.Row
	rows                        pgx.Rows
	closes                      int
	closeFn                     func()
}

func (b *batchStub) Exec() (pgconn.CommandTag, error) { return pgconn.CommandTag{}, b.execErr }
func (b *batchStub) Query() (pgx.Rows, error)         { return b.rows, b.queryErr }
func (b *batchStub) QueryRow() pgx.Row                { return b.row }
func (b *batchStub) Close() error {
	b.closes++
	if b.closeFn != nil {
		b.closeFn()
	}
	return b.closeErr
}

type batchCollector struct {
	calls int
	err   error
	ctx   context.Context
}

func (c *batchCollector) TrackQueryMetrics(ctx context.Context, _ time.Time, err error) {
	c.calls++
	c.err = err
	c.ctx = ctx
}

func TestBatchMetricsErrorsAndClose(t *testing.T) {
	failure := errors.New("batch failure")
	for _, operation := range []string{"success", "exec", "query", "row scan", "rows scan", "rows values", "rows next", "rows close", "rows err", "close"} {
		t.Run(operation, func(t *testing.T) {
			pool, collector := NewMockPool(t), &batchCollector{}
			stub := &batchStub{}
			batch := &pgx.Batch{}
			var sentCtx context.Context
			pool.EXPECT().SendBatch(mock.Anything, batch).RunAndReturn(func(ctx context.Context, _ *pgx.Batch) pgx.BatchResults { sentCtx = ctx; return stub }).Once()
			ctx := pgcontext.With(context.Background(), pgcontext.WithTimeout(time.Second), pgcontext.WithMetricsLabel("batch"))
			results := New(pool, collector).SendBatch(ctx, batch)
			require.NotNil(t, sentCtx)
			_, deadline := sentCtx.Deadline()
			require.True(t, deadline)
			switch operation {
			case "exec":
				stub.execErr = failure
				_, err := results.Exec()
				require.ErrorIs(t, err, failure)
			case "query":
				stub.queryErr = failure
				_, err := results.Query()
				require.ErrorIs(t, err, failure)
			case "row scan":
				row := NewMockRow(t)
				row.EXPECT().Scan(mock.Anything).Return(failure).Once()
				stub.row = row
				var value int
				require.ErrorIs(t, results.QueryRow().Scan(&value), failure)
			case "rows scan", "rows values", "rows next", "rows close", "rows err":
				rows := NewMockRows(t)
				stub.rows = rows
				wrapped, err := results.Query()
				require.NoError(t, err)
				switch operation {
				case "rows scan":
					rows.EXPECT().Scan(mock.Anything).Return(failure).Once()
					var value int
					require.ErrorIs(t, wrapped.Scan(&value), failure)
				case "rows values":
					rows.EXPECT().Values().Return(nil, failure).Once()
					_, err = wrapped.Values()
					require.ErrorIs(t, err, failure)
				case "rows next":
					rows.EXPECT().Next().Return(false).Once()
					rows.EXPECT().Err().Return(failure).Once()
					require.False(t, wrapped.Next())
				case "rows close":
					rows.EXPECT().Close().Once()
					rows.EXPECT().Err().Return(failure).Once()
					wrapped.Close()
				case "rows err":
					rows.EXPECT().Err().Return(failure).Once()
					require.ErrorIs(t, wrapped.Err(), failure)
				}
			case "close":
				stub.closeErr = failure
			}
			require.Zero(t, collector.calls)
			require.NoError(t, sentCtx.Err(), "reading a result must not cancel the batch")
			stub.closeFn = func() { require.NoError(t, sentCtx.Err(), "Close must drain before cancellation") }
			closeErr := results.Close()
			if operation == "close" {
				require.ErrorIs(t, closeErr, failure)
			} else {
				require.NoError(t, closeErr)
			}
			require.Equal(t, closeErr, results.Close())
			require.Equal(t, 1, stub.closes)
			require.Equal(t, 1, collector.calls)
			require.ErrorIs(t, sentCtx.Err(), context.Canceled)
			if operation == "success" {
				require.NoError(t, collector.err)
			} else {
				require.ErrorIs(t, collector.err, failure)
			}
			labels, ok := pgcontext.MetricsLabelsFrom(collector.ctx)
			require.True(t, ok)
			require.Equal(t, []string{"batch"}, labels)
		})
	}
}

func TestBatchTimeoutPassedBeforeSending(t *testing.T) {
	for _, kind := range []string{"timeout", "canceled parent"} {
		t.Run(kind, func(t *testing.T) {
			pool, collector := NewMockPool(t), &batchCollector{}
			ctx := context.Background()
			expected := context.DeadlineExceeded
			if kind == "timeout" {
				ctx = pgcontext.With(ctx, pgcontext.WithTimeout(-time.Second))
			} else {
				canceled, cancel := context.WithCancel(ctx)
				cancel()
				ctx = canceled
				expected = context.Canceled
			}
			stub := &batchStub{closeErr: expected}
			batch := &pgx.Batch{}
			pool.EXPECT().SendBatch(mock.Anything, batch).RunAndReturn(func(sent context.Context, _ *pgx.Batch) pgx.BatchResults {
				require.ErrorIs(t, sent.Err(), expected)
				return stub
			}).Once()
			results := New(pool, collector).SendBatch(ctx, batch)
			require.ErrorIs(t, results.Close(), expected)
			require.Equal(t, 1, collector.calls)
			require.ErrorIs(t, collector.err, expected)
		})
	}
}

func TestBatchRowsCloseKeepsFollowingResultsUsable(t *testing.T) {
	pool, collector := NewMockPool(t), &batchCollector{}
	rows := NewMockRows(t)
	rows.EXPECT().Close().Once()
	rows.EXPECT().Err().Return(nil).Once()
	stub := &batchStub{rows: rows}
	batch := &pgx.Batch{}
	var sentCtx context.Context
	pool.EXPECT().SendBatch(mock.Anything, batch).RunAndReturn(func(ctx context.Context, _ *pgx.Batch) pgx.BatchResults { sentCtx = ctx; return stub }).Once()
	results := New(pool, collector).SendBatch(pgcontext.With(context.Background(), pgcontext.WithTimeout(time.Second)), batch)
	wrapped, err := results.Query()
	require.NoError(t, err)
	wrapped.Close()
	require.NoError(t, sentCtx.Err())
	_, err = results.Exec()
	require.NoError(t, err)
	require.Zero(t, collector.calls)
	require.NoError(t, results.Close())
	require.Equal(t, 1, collector.calls)
}

func TestBatchParentDeadlineTakesPrecedence(t *testing.T) {
	pool, collector := NewMockPool(t), &batchCollector{}
	parent, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	deadline, _ := parent.Deadline()
	ctx := pgcontext.With(parent, pgcontext.WithTimeout(time.Hour))
	batch := &pgx.Batch{}
	stub := &batchStub{}
	pool.EXPECT().SendBatch(mock.Anything, batch).RunAndReturn(func(sent context.Context, _ *pgx.Batch) pgx.BatchResults {
		actual, ok := sent.Deadline()
		require.True(t, ok)
		require.Equal(t, deadline, actual)
		return stub
	}).Once()
	require.NoError(t, New(pool, collector).SendBatch(ctx, batch).Close())
	require.NoError(t, parent.Err(), "batch Close must not cancel the parent")
}

func TestBatchMetricsRealErrorReplacesNoRows(t *testing.T) {
	failure := errors.New("batch failure")
	for _, order := range []string{"no rows only", "no rows then failure", "failure then no rows"} {
		t.Run(order, func(t *testing.T) {
			pool, collector := NewMockPool(t), &batchCollector{}
			row := NewMockRow(t)
			row.EXPECT().Scan(mock.Anything).Return(pgx.ErrNoRows).Once()
			stub := &batchStub{row: row, execErr: failure}
			batch := &pgx.Batch{}
			pool.EXPECT().SendBatch(mock.Anything, batch).Return(stub).Once()
			results := New(pool, collector).SendBatch(context.Background(), batch)
			var value int
			switch order {
			case "no rows only":
				require.ErrorIs(t, results.QueryRow().Scan(&value), pgx.ErrNoRows)
			case "no rows then failure":
				require.ErrorIs(t, results.QueryRow().Scan(&value), pgx.ErrNoRows)
				_, err := results.Exec()
				require.ErrorIs(t, err, failure)
			case "failure then no rows":
				_, err := results.Exec()
				require.ErrorIs(t, err, failure)
				require.ErrorIs(t, results.QueryRow().Scan(&value), pgx.ErrNoRows)
			}
			require.NoError(t, results.Close())
			if order == "no rows only" {
				require.ErrorIs(t, collector.err, pgx.ErrNoRows)
			} else {
				require.ErrorIs(t, collector.err, failure)
			}
		})
	}
}
