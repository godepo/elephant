package acceptance

import (
	"context"
	"github.com/godepo/elephant"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestQueryTimeout(t *testing.T) {
	t.Run("should interrupt Exec on PostgreSQL and release its connection", func(t *testing.T) {
		tcs := suite.Case(t)
		parent, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		ctx := elephant.With(parent, elephant.WithTimeout(50*time.Millisecond))
		_, err := tcs.SUT.Exec(ctx, "SELECT pg_sleep(1)")
		require.ErrorIs(t, err, context.DeadlineExceeded)
		require.NoError(t, parent.Err())
		require.Equal(t, 1, tcs.Deps.Collector.calls)
		require.ErrorIs(t, tcs.Deps.Collector.err, context.DeadlineExceeded)
		requireReleased(t, tcs.Deps)
		var value int
		require.NoError(t, tcs.SUT.QueryRow(parent, "SELECT 1").Scan(&value))
		require.Equal(t, 1, value)
	})
	t.Run("should interrupt Query on PostgreSQL and release its connection", func(t *testing.T) {
		tcs := suite.Case(t)
		parent, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		ctx := elephant.With(parent, elephant.WithTimeout(50*time.Millisecond))
		rows, err := tcs.SUT.Query(ctx, "SELECT pg_sleep(1)")
		err = queryTimeoutError(rows, err)
		require.ErrorIs(t, err, context.DeadlineExceeded)
		require.NoError(t, parent.Err())
		require.Equal(t, 1, tcs.Deps.Collector.calls)
		require.ErrorIs(t, tcs.Deps.Collector.err, context.DeadlineExceeded)
		requireReleased(t, tcs.Deps)
		var value int
		require.NoError(t, tcs.SUT.QueryRow(parent, "SELECT 1").Scan(&value))
		require.Equal(t, 1, value)
	})
	t.Run("should interrupt QueryRow on PostgreSQL and release its connection", func(t *testing.T) {
		tcs := suite.Case(t)
		parent, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		ctx := elephant.With(parent, elephant.WithTimeout(50*time.Millisecond))
		err := tcs.SUT.QueryRow(ctx, "SELECT pg_sleep(1)").Scan()
		require.ErrorIs(t, err, context.DeadlineExceeded)
		require.NoError(t, parent.Err())
		require.Equal(t, 1, tcs.Deps.Collector.calls)
		require.ErrorIs(t, tcs.Deps.Collector.err, context.DeadlineExceeded)
		requireReleased(t, tcs.Deps)
		var value int
		require.NoError(t, tcs.SUT.QueryRow(parent, "SELECT 1").Scan(&value))
		require.Equal(t, 1, value)
	})
}

// Query can surface a deadline either during dispatch or while reading results.
func queryTimeoutError(rows pgx.Rows, err error) error {
	if err != nil {
		return err
	}
	_, err = pgx.CollectRows(rows, func(pgx.CollectableRow) (int, error) { return 0, nil })
	return err
}
