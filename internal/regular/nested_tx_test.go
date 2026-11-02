package regular

import (
	"context"
	"fmt"
	"github.com/godepo/elephant/internal/pkg/pgcontext"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
	"testing"
)

// Real PostgreSQL success and rollback paths are covered in regular_test.go.
// pgx closes a nested transaction after Commit, so a subsequent Rollback cannot
// produce anything other than ErrTxClosed. Mocks exercise those exceptional paths.
func TestInstance_Transactional_NestedErrors(t *testing.T) {
	t.Run("should preserve callback and rollback errors", func(t *testing.T) {
		tcs := mainProvider(t)
		tcs.Go()
		tcs.Given(ArrangeContext, ArrangeTxMockInContext, ArrangeNestedTx).When(ActBeginNested)
		callbackErr := &pgconn.PgError{Message: "business rule rejected", Code: "P0001"}
		rollbackErr := &pgconn.PgError{Message: "rollback failed", Code: "08006"}
		tcs.State.ctx = pgcontext.With(tcs.State.ctx, pgcontext.WithFnTxPassMatcher(func(ctx context.Context, err error) bool {
			tx, ok := pgcontext.TransactionFrom(ctx)
			require.True(t, ok)
			require.Same(t, tcs.State.TxMock, tx)
			require.ErrorIs(t, err, callbackErr)
			return false
		}))
		tcs.State.NestedTxMock.EXPECT().Rollback(tcs.State.ctx).Return(rollbackErr).Once()
		err := tcs.SUT.Transactional(tcs.State.ctx, func(ctx context.Context) error {
			tx, ok := pgcontext.TransactionFrom(ctx)
			require.True(t, ok)
			require.Same(t, tcs.State.NestedTxMock, tx)
			return callbackErr
		})
		require.ErrorIs(t, err, callbackErr)
		var cause *pgconn.PgError
		require.ErrorAs(t, err, &cause)
		require.Same(t, callbackErr, cause)
		require.ErrorIs(t, err, rollbackErr)
		require.Contains(t, err.Error(), "can't rollback nested transaction")
	})
	t.Run("should preserve commit and rollback errors", func(t *testing.T) {
		tcs := mainProvider(t)
		tcs.Go()
		tcs.Given(ArrangeContext, ArrangeTxMockInContext, ArrangeNestedTx).When(ActBeginNested)
		commitErr := &pgconn.PgError{Message: "commit failed", Code: "40001"}
		rollbackErr := &pgconn.PgError{Message: "rollback failed", Code: "08006"}
		tcs.State.NestedTxMock.EXPECT().Commit(tcs.State.ctx).Return(commitErr).Once()
		tcs.State.NestedTxMock.EXPECT().Rollback(tcs.State.ctx).Return(rollbackErr).Once()
		err := tcs.SUT.Transactional(tcs.State.ctx, func(ctx context.Context) error {
			tx, ok := pgcontext.TransactionFrom(ctx)
			require.True(t, ok)
			require.Same(t, tcs.State.NestedTxMock, tx)
			return nil
		})
		require.ErrorIs(t, err, commitErr)
		var cause *pgconn.PgError
		require.ErrorAs(t, err, &cause)
		require.Same(t, commitErr, cause)
		require.ErrorIs(t, err, rollbackErr)
		require.Contains(t, err.Error(), "can't rollback nested transaction")
		require.Contains(t, err.Error(), "can't commit nested transaction")
	})
	t.Run("should report rollback failure after a successful commit", func(t *testing.T) {
		tcs := mainProvider(t)
		tcs.Go()
		tcs.Given(ArrangeContext, ArrangeTxMockInContext, ArrangeNestedTx).When(ActBeginNested)
		rollbackErr := &pgconn.PgError{Message: "rollback failed", Code: "08006"}
		tcs.State.NestedTxMock.EXPECT().Commit(tcs.State.ctx).Return(nil).Once()
		tcs.State.NestedTxMock.EXPECT().Rollback(tcs.State.ctx).Return(rollbackErr).Once()
		err := tcs.SUT.Transactional(tcs.State.ctx, func(ctx context.Context) error {
			tx, ok := pgcontext.TransactionFrom(ctx)
			require.True(t, ok)
			require.Same(t, tcs.State.NestedTxMock, tx)
			return nil
		})
		require.ErrorIs(t, err, rollbackErr)
		var cause *pgconn.PgError
		require.ErrorAs(t, err, &cause)
		require.Same(t, rollbackErr, cause)
		require.ErrorIs(t, err, rollbackErr)
		require.Contains(t, err.Error(), "can't rollback nested transaction")
	})
	t.Run("should preserve a matched callback when rollback fails", func(t *testing.T) {
		tcs := mainProvider(t)
		tcs.Go()
		tcs.Given(ArrangeContext, ArrangeTxMockInContext, ArrangeNestedTx).When(ActBeginNested)
		callbackErr := &pgconn.PgError{Message: "business rule rejected", Code: "P0001"}
		rollbackErr := &pgconn.PgError{Message: "rollback failed", Code: "08006"}
		tcs.State.ctx = pgcontext.With(tcs.State.ctx, pgcontext.WithFnTxPassMatcher(func(ctx context.Context, err error) bool {
			tx, ok := pgcontext.TransactionFrom(ctx)
			require.True(t, ok)
			require.Same(t, tcs.State.TxMock, tx)
			require.ErrorIs(t, err, callbackErr)
			return true
		}))
		tcs.State.NestedTxMock.EXPECT().Commit(tcs.State.ctx).Return(nil).Once()
		tcs.State.NestedTxMock.EXPECT().Rollback(tcs.State.ctx).Return(rollbackErr).Once()
		err := tcs.SUT.Transactional(tcs.State.ctx, func(ctx context.Context) error {
			tx, ok := pgcontext.TransactionFrom(ctx)
			require.True(t, ok)
			require.Same(t, tcs.State.NestedTxMock, tx)
			return callbackErr
		})
		require.ErrorIs(t, err, callbackErr)
		var cause *pgconn.PgError
		require.ErrorAs(t, err, &cause)
		require.Same(t, callbackErr, cause)
		require.ErrorIs(t, err, rollbackErr)
		require.Contains(t, err.Error(), "can't rollback nested transaction")
	})
	t.Run("should preserve commit and rollback errors after a matched callback", func(t *testing.T) {
		tcs := mainProvider(t)
		tcs.Go()
		tcs.Given(ArrangeContext, ArrangeTxMockInContext, ArrangeNestedTx).When(ActBeginNested)
		callbackErr := &pgconn.PgError{Message: "business rule rejected", Code: "P0001"}
		commitErr := &pgconn.PgError{Message: "commit failed", Code: "40001"}
		rollbackErr := &pgconn.PgError{Message: "rollback failed", Code: "08006"}
		tcs.State.ctx = pgcontext.With(tcs.State.ctx, pgcontext.WithFnTxPassMatcher(func(ctx context.Context, err error) bool {
			tx, ok := pgcontext.TransactionFrom(ctx)
			require.True(t, ok)
			require.Same(t, tcs.State.TxMock, tx)
			require.ErrorIs(t, err, callbackErr)
			return true
		}))
		tcs.State.NestedTxMock.EXPECT().Commit(tcs.State.ctx).Return(commitErr).Once()
		tcs.State.NestedTxMock.EXPECT().Rollback(tcs.State.ctx).Return(rollbackErr).Once()
		err := tcs.SUT.Transactional(tcs.State.ctx, func(ctx context.Context) error {
			tx, ok := pgcontext.TransactionFrom(ctx)
			require.True(t, ok)
			require.Same(t, tcs.State.NestedTxMock, tx)
			return callbackErr
		})
		require.ErrorIs(t, err, commitErr)
		var cause *pgconn.PgError
		require.ErrorAs(t, err, &cause)
		require.Same(t, commitErr, cause)
		require.ErrorIs(t, err, rollbackErr)
		require.Contains(t, err.Error(), "can't rollback nested transaction")
		require.Contains(t, err.Error(), "can't commit nested transaction")
		require.NotErrorIs(t, err, callbackErr, "a commit failure replaces a matched callback error")
	})
	t.Run("should ignore wrapped ErrTxClosed after callback failure", func(t *testing.T) {
		tcs := mainProvider(t)
		tcs.Go()
		tcs.Given(ArrangeContext, ArrangeTxMockInContext, ArrangeNestedTx).When(ActBeginNested)
		callbackErr := &pgconn.PgError{Message: "business rule rejected", Code: "P0001"}
		rollbackErr := fmt.Errorf("transaction completed: %w", pgx.ErrTxClosed)
		tcs.State.ctx = pgcontext.With(tcs.State.ctx, pgcontext.WithFnTxPassMatcher(func(ctx context.Context, err error) bool {
			tx, ok := pgcontext.TransactionFrom(ctx)
			require.True(t, ok)
			require.Same(t, tcs.State.TxMock, tx)
			require.ErrorIs(t, err, callbackErr)
			return false
		}))
		tcs.State.NestedTxMock.EXPECT().Rollback(tcs.State.ctx).Return(rollbackErr).Once()
		err := tcs.SUT.Transactional(tcs.State.ctx, func(ctx context.Context) error {
			tx, ok := pgcontext.TransactionFrom(ctx)
			require.True(t, ok)
			require.Same(t, tcs.State.NestedTxMock, tx)
			return callbackErr
		})
		require.ErrorIs(t, err, callbackErr)
		var cause *pgconn.PgError
		require.ErrorAs(t, err, &cause)
		require.Same(t, callbackErr, cause)
		require.NotErrorIs(t, err, pgx.ErrTxClosed)
	})
	t.Run("should ignore wrapped ErrTxClosed after commit failure", func(t *testing.T) {
		tcs := mainProvider(t)
		tcs.Go()
		tcs.Given(ArrangeContext, ArrangeTxMockInContext, ArrangeNestedTx).When(ActBeginNested)
		commitErr := &pgconn.PgError{Message: "commit failed", Code: "40001"}
		rollbackErr := fmt.Errorf("transaction completed: %w", pgx.ErrTxClosed)
		tcs.State.NestedTxMock.EXPECT().Commit(tcs.State.ctx).Return(commitErr).Once()
		tcs.State.NestedTxMock.EXPECT().Rollback(tcs.State.ctx).Return(rollbackErr).Once()
		err := tcs.SUT.Transactional(tcs.State.ctx, func(ctx context.Context) error {
			tx, ok := pgcontext.TransactionFrom(ctx)
			require.True(t, ok)
			require.Same(t, tcs.State.NestedTxMock, tx)
			return nil
		})
		require.ErrorIs(t, err, commitErr)
		var cause *pgconn.PgError
		require.ErrorAs(t, err, &cause)
		require.Same(t, commitErr, cause)
		require.NotErrorIs(t, err, pgx.ErrTxClosed)
		require.Contains(t, err.Error(), "can't commit nested transaction")
	})
}
