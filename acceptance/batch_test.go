package acceptance

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/godepo/elephant"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
)

// requireReleased waits briefly, because the pool destroys a connection broken by a timeout asynchronously.
func requireReleased(t *testing.T, deps Deps) {
	t.Helper()
	require.Eventually(t, func() bool { return deps.Pool.Stat().AcquiredConns() == 0 },
		time.Second, 10*time.Millisecond, "closed results must release the connection")
}

func TestSendBatch(t *testing.T) {
	ctx := context.Background()

	t.Run("results", func(t *testing.T) {
		tcs := suite.Case(t)
		batch := &pgx.Batch{}
		batch.Queue("INSERT INTO numbers (value) VALUES ($1), ($2)", 1, 2)
		batch.Queue("SELECT value FROM numbers ORDER BY value")
		batch.Queue("SELECT sum(value) FROM numbers")
		results := tcs.SUT.SendBatch(ctx, batch)
		tag, err := results.Exec()
		require.NoError(t, err)
		require.EqualValues(t, 2, tag.RowsAffected())
		rows, err := results.Query()
		require.NoError(t, err)
		values, err := pgx.CollectRows(rows, pgx.RowTo[int32])
		require.NoError(t, err)
		require.Equal(t, []int32{1, 2}, values)
		var sum int64
		require.NoError(t, results.QueryRow().Scan(&sum))
		require.EqualValues(t, 3, sum)
		require.Zero(t, tcs.Deps.Collector.calls, "a batch is measured when it is closed")
		require.NoError(t, results.Close())
		require.Equal(t, 1, tcs.Deps.Collector.calls)
		require.NoError(t, tcs.Deps.Collector.err)
		requireReleased(t, tcs.Deps)
	})

	t.Run("transaction context", func(t *testing.T) {
		tcs := suite.Case(t)
		rollback := errors.New("rollback")
		err := tcs.SUT.Transactional(ctx, func(txCtx context.Context) error {
			tx, ok := elephant.TransactionFrom(txCtx)
			require.True(t, ok)
			var txPID, pid int32
			require.NoError(t, tx.QueryRow(txCtx, "SELECT pg_backend_pid()").Scan(&txPID))
			batch := &pgx.Batch{}
			batch.Queue("SELECT pg_backend_pid()")
			batch.Queue("INSERT INTO numbers (value) VALUES (1)")
			results := tcs.SUT.SendBatch(txCtx, batch)
			require.NoError(t, results.QueryRow().Scan(&pid))
			require.Equal(t, txPID, pid)
			_, err := results.Exec()
			require.NoError(t, err)
			require.NoError(t, results.Close())
			return rollback
		})
		require.ErrorIs(t, err, rollback)
		var count int64
		require.NoError(t, tcs.SUT.QueryRow(ctx, "SELECT count(*) FROM numbers").Scan(&count))
		require.Zero(t, count, "the batch is rolled back with the transaction")
		requireReleased(t, tcs.Deps)
	})

	t.Run("callbacks and drain", func(t *testing.T) {
		tcs := suite.Case(t)
		batch := &pgx.Batch{}
		calls := 0
		batch.Queue("SELECT 1").Exec(func(tag pgconn.CommandTag) error {
			calls++
			require.EqualValues(t, 1, tag.RowsAffected())
			return nil
		})
		batch.Queue("SELECT 2").Query(func(rows pgx.Rows) error {
			calls++
			require.True(t, rows.Next())
			var value int
			require.NoError(t, rows.Scan(&value))
			require.Equal(t, 2, value)
			return nil
		})
		batch.Queue("SELECT 3").QueryRow(func(row pgx.Row) error {
			calls++
			var value int
			require.NoError(t, row.Scan(&value))
			require.Equal(t, 3, value)
			return nil
		})
		results := tcs.SUT.SendBatch(ctx, batch)
		require.NoError(t, results.Close())
		require.NoError(t, results.Close())
		require.Equal(t, 3, calls)
		require.Equal(t, 1, tcs.Deps.Collector.calls)
		require.NoError(t, tcs.Deps.Collector.err)
		requireReleased(t, tcs.Deps)
	})

	t.Run("callback error", func(t *testing.T) {
		tcs := suite.Case(t)
		failure := errors.New("callback failed")
		batch := &pgx.Batch{}
		batch.Queue("SELECT 1").Exec(func(pgconn.CommandTag) error { return failure })
		batch.Queue("SELECT 2").Exec(func(pgconn.CommandTag) error { t.Error("callback after error"); return nil })
		results := tcs.SUT.SendBatch(ctx, batch)
		require.ErrorIs(t, results.Close(), failure)
		require.ErrorIs(t, results.Close(), failure)
		require.ErrorIs(t, tcs.Deps.Collector.err, failure)
		requireReleased(t, tcs.Deps)
	})

	t.Run("unread SQL error", func(t *testing.T) {
		tcs := suite.Case(t)
		batch := &pgx.Batch{}
		batch.Queue("SELECT 1")
		batch.Queue("SELECT 1 / $1::integer", 0)
		results := tcs.SUT.SendBatch(ctx, batch)
		_, err := results.Exec()
		require.NoError(t, err)
		err = results.Close()
		var pgErr *pgconn.PgError
		require.ErrorAs(t, err, &pgErr)
		require.Equal(t, "22012", pgErr.Code)
		require.ErrorIs(t, tcs.Deps.Collector.err, err)
		require.ErrorIs(t, results.Close(), err)
		requireReleased(t, tcs.Deps)
	})

	t.Run("no rows reported in metrics", func(t *testing.T) {
		tcs := suite.Case(t)
		batch := &pgx.Batch{}
		batch.Queue("SELECT 1 WHERE false")
		batch.Queue("SELECT 2")
		results := tcs.SUT.SendBatch(ctx, batch)
		var value int
		require.ErrorIs(t, results.QueryRow().Scan(&value), pgx.ErrNoRows)
		require.NoError(t, results.QueryRow().Scan(&value))
		require.Equal(t, 2, value)
		require.NoError(t, results.Close())
		require.ErrorIs(t, tcs.Deps.Collector.err, pgx.ErrNoRows)
		requireReleased(t, tcs.Deps)
	})

	t.Run("timeout during query", func(t *testing.T) {
		tcs := suite.Case(t)
		batch := &pgx.Batch{}
		batch.Queue("SELECT pg_sleep(1)")
		results := tcs.SUT.SendBatch(elephant.With(ctx, elephant.WithTimeout(20*time.Millisecond)), batch)
		_, err := results.Exec()
		require.ErrorIs(t, err, context.DeadlineExceeded)
		require.Error(t, results.Close())
		require.ErrorIs(t, tcs.Deps.Collector.err, context.DeadlineExceeded)
		requireReleased(t, tcs.Deps)
	})

	t.Run("rows close leaves batch alive", func(t *testing.T) {
		tcs := suite.Case(t)
		batch := &pgx.Batch{}
		batch.Queue("SELECT 1")
		batch.Queue("SELECT 2")
		results := tcs.SUT.SendBatch(elephant.With(ctx, elephant.WithTimeout(time.Second)), batch)
		rows, err := results.Query()
		require.NoError(t, err)
		rows.Close()
		var value int
		require.NoError(t, results.QueryRow().Scan(&value))
		require.Equal(t, 2, value)
		require.NoError(t, results.Close())
		requireReleased(t, tcs.Deps)
	})

	t.Run("empty batch", func(t *testing.T) {
		tcs := suite.Case(t)
		results := tcs.SUT.SendBatch(ctx, &pgx.Batch{})
		require.NoError(t, results.Close())
		require.Equal(t, 1, tcs.Deps.Collector.calls)
		requireReleased(t, tcs.Deps)
	})
}
