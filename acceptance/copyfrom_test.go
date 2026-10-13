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

// countNumbers reads through db, so a transaction sees its own uncommitted rows.
func countNumbers(t *testing.T, ctx context.Context, db interface {
	QueryRow(ctx context.Context, query string, args ...any) pgx.Row
}) (count, sum int64) {
	t.Helper()
	require.NoError(t, db.QueryRow(ctx, "SELECT count(*), coalesce(sum(value), 0) FROM numbers").Scan(&count, &sum))
	return count, sum
}

func TestCopyFrom(t *testing.T) {
	ctx := context.Background()
	table, columns := pgx.Identifier{"numbers"}, []string{"value"}

	t.Run("rows", func(t *testing.T) {
		tcs := suite.Case(t)
		copied, err := tcs.SUT.CopyFrom(ctx, table, columns, pgx.CopyFromRows([][]any{{1}, {2}, {3}}))
		require.NoError(t, err)
		require.EqualValues(t, 3, copied)
		require.Equal(t, 1, tcs.Deps.Collector.calls)
		require.NoError(t, tcs.Deps.Collector.err)
		count, sum := countNumbers(t, ctx, tcs.SUT)
		require.EqualValues(t, 3, count)
		require.EqualValues(t, 6, sum)
		requireReleased(t, tcs.Deps)
	})

	t.Run("transaction context", func(t *testing.T) {
		tcs := suite.Case(t)
		rollback := errors.New("rollback")
		err := tcs.SUT.Transactional(ctx, func(txCtx context.Context) error {
			tx, ok := elephant.TransactionFrom(txCtx)
			require.True(t, ok)
			copied, err := tcs.SUT.CopyFrom(txCtx, table, columns, pgx.CopyFromRows([][]any{{4}, {5}}))
			require.NoError(t, err)
			require.EqualValues(t, 2, copied)
			count, _ := countNumbers(t, txCtx, tx)
			require.EqualValues(t, 2, count, "the copy is visible in its transaction")
			return rollback
		})
		require.ErrorIs(t, err, rollback)
		count, _ := countNumbers(t, ctx, tcs.SUT)
		require.Zero(t, count, "the copy is rolled back with the transaction")
		requireReleased(t, tcs.Deps)
	})

	t.Run("source error", func(t *testing.T) {
		tcs := suite.Case(t)
		failure := errors.New("source failed")
		rows := 0
		source := pgx.CopyFromFunc(func() ([]any, error) {
			rows++
			if rows > 1 {
				return nil, failure
			}
			return []any{int32(1)}, nil
		})
		copied, err := tcs.SUT.CopyFrom(ctx, table, columns, source)
		// pgx aborts the copy with the source error's message and returns the server's error.
		var pgErr *pgconn.PgError
		require.ErrorAs(t, err, &pgErr)
		require.Equal(t, "57014", pgErr.Code)
		require.Contains(t, pgErr.Message, failure.Error())
		require.Zero(t, copied)
		require.ErrorIs(t, tcs.Deps.Collector.err, pgErr)
		count, _ := countNumbers(t, ctx, tcs.SUT)
		require.Zero(t, count)
		requireReleased(t, tcs.Deps)
	})

	t.Run("timeout", func(t *testing.T) {
		tcs := suite.Case(t)
		sent := false
		source := pgx.CopyFromFunc(func() ([]any, error) {
			if sent {
				return nil, nil
			}
			sent = true
			time.Sleep(200 * time.Millisecond)
			return []any{int32(1)}, nil
		})
		timeoutCtx := elephant.With(ctx, elephant.WithTimeout(20*time.Millisecond))
		_, err := tcs.SUT.CopyFrom(timeoutCtx, table, columns, source)
		require.ErrorIs(t, err, context.DeadlineExceeded)
		require.ErrorIs(t, tcs.Deps.Collector.err, context.DeadlineExceeded)
		count, _ := countNumbers(t, ctx, tcs.SUT)
		require.Zero(t, count)
		requireReleased(t, tcs.Deps)
	})
}
