package metrics

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/godepo/elephant/internal/pkg/pgcontext"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestCopyFromMetrics(t *testing.T) {
	t.Run("should record copy success once and preserve labels", func(t *testing.T) {
		pool, collector := NewMockPool(t), &batchCollector{}
		table, columns, source := pgx.Identifier{"values"}, []string{"value"}, pgx.CopyFromRows([][]any{{1}})
		ctx := pgcontext.With(context.Background(), pgcontext.WithMetricsLabel("copy"))
		var expected error
		pool.EXPECT().CopyFrom(ctx, table, columns, source).Return(1, expected).Once()
		copied, err := New(pool, collector).CopyFrom(ctx, table, columns, source)
		require.ErrorIs(t, err, expected)
		require.EqualValues(t, 1, copied)
		require.Equal(t, 1, collector.calls)
		require.ErrorIs(t, collector.err, expected)
		labels, ok := pgcontext.MetricsLabelsFrom(collector.ctx)
		require.True(t, ok)
		require.Equal(t, []string{"copy"}, labels)
	})
	t.Run("should record copy error once and preserve labels", func(t *testing.T) {
		pool, collector := NewMockPool(t), &batchCollector{}
		table, columns, source := pgx.Identifier{"values"}, []string{"value"}, pgx.CopyFromRows([][]any{{1}})
		ctx := pgcontext.With(context.Background(), pgcontext.WithMetricsLabel("copy"))
		expected := errors.New("copy failed")
		pool.EXPECT().CopyFrom(ctx, table, columns, source).Return(1, expected).Once()
		copied, err := New(pool, collector).CopyFrom(ctx, table, columns, source)
		require.ErrorIs(t, err, expected)
		require.EqualValues(t, 1, copied)
		require.Equal(t, 1, collector.calls)
		require.ErrorIs(t, collector.err, expected)
		labels, ok := pgcontext.MetricsLabelsFrom(collector.ctx)
		require.True(t, ok)
		require.Equal(t, []string{"copy"}, labels)
	})
}

func TestCopyFromTimeoutPassedBeforeCopy(t *testing.T) {
	pool, collector := NewMockPool(t), &batchCollector{}
	ctx := pgcontext.With(context.Background(), pgcontext.WithTimeout(time.Second))
	var sentCtx context.Context
	pool.EXPECT().CopyFrom(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		RunAndReturn(func(ctx context.Context, _ pgx.Identifier, _ []string, _ pgx.CopyFromSource) (int64, error) {
			sentCtx = ctx
			_, ok := ctx.Deadline()
			require.True(t, ok, "the timeout must be applied before the copy starts")
			require.NoError(t, ctx.Err())
			return 0, nil
		}).Once()
	_, err := New(pool, collector).CopyFrom(ctx, pgx.Identifier{"values"}, nil, pgx.CopyFromRows(nil))
	require.NoError(t, err)
	require.ErrorIs(t, sentCtx.Err(), context.Canceled, "the timeout context is canceled after the copy")
	require.Equal(t, 1, collector.calls)
}

func TestCopyFromZeroTimeoutExpiresBeforeDispatch(t *testing.T) {
	pool, collector := NewMockPool(t), &batchCollector{}
	parent := context.Background()
	ctx := pgcontext.With(parent, pgcontext.WithTimeout(0))
	pool.EXPECT().CopyFrom(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		RunAndReturn(func(sent context.Context, _ pgx.Identifier, _ []string, _ pgx.CopyFromSource) (int64, error) {
			require.ErrorIs(t, sent.Err(), context.DeadlineExceeded)
			return 0, sent.Err()
		}).Once()
	copied, err := New(pool, collector).CopyFrom(ctx, pgx.Identifier{"values"}, nil, pgx.CopyFromRows(nil))
	require.Zero(t, copied)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Equal(t, 1, collector.calls)
	require.ErrorIs(t, collector.err, context.DeadlineExceeded)
	require.NoError(t, parent.Err())
}

func TestCopyFromNegativeTimeoutExpiresBeforeDispatch(t *testing.T) {
	pool, collector := NewMockPool(t), &batchCollector{}
	parent := context.Background()
	ctx := pgcontext.With(parent, pgcontext.WithTimeout(-time.Second))
	pool.EXPECT().CopyFrom(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		RunAndReturn(func(sent context.Context, _ pgx.Identifier, _ []string, _ pgx.CopyFromSource) (int64, error) {
			require.ErrorIs(t, sent.Err(), context.DeadlineExceeded)
			return 0, sent.Err()
		}).Once()
	copied, err := New(pool, collector).CopyFrom(ctx, pgx.Identifier{"values"}, nil, pgx.CopyFromRows(nil))
	require.Zero(t, copied)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Equal(t, 1, collector.calls)
	require.ErrorIs(t, collector.err, context.DeadlineExceeded)
	require.NoError(t, parent.Err())
}

func TestCopyFromParentDeadlineTakesPrecedence(t *testing.T) {
	pool, collector := NewMockPool(t), &batchCollector{}
	parent, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	deadline, _ := parent.Deadline()
	ctx := pgcontext.With(parent, pgcontext.WithTimeout(time.Hour))
	var sentCtx context.Context
	pool.EXPECT().CopyFrom(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		RunAndReturn(func(sent context.Context, _ pgx.Identifier, _ []string, _ pgx.CopyFromSource) (int64, error) {
			sentCtx = sent
			actual, ok := sent.Deadline()
			require.True(t, ok)
			require.Equal(t, deadline, actual)
			return 0, errors.New("copy failed")
		}).Once()
	_, err := New(pool, collector).CopyFrom(ctx, pgx.Identifier{"values"}, nil, pgx.CopyFromRows(nil))
	require.Error(t, err)
	require.ErrorIs(t, sentCtx.Err(), context.Canceled)
	require.NoError(t, parent.Err())
	require.Equal(t, 1, collector.calls)
	require.ErrorIs(t, collector.err, err)
}
