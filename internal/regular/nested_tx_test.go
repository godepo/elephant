package regular

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/godepo/elephant/internal/pkg/pgcontext"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
)

func TestInstance_Transactional_NestedErrors(t *testing.T) {
	callbackErr := &pgconn.PgError{Message: "business rule rejected", Code: "P0001"}
	commitErr := &pgconn.PgError{Message: "commit failed", Code: "40001"}
	rollbackErr := &pgconn.PgError{Message: "rollback failed", Code: "08006"}
	closedErr := fmt.Errorf("transaction completed: %w", pgx.ErrTxClosed)

	tests := []struct {
		name        string
		callbackErr error
		commitErr   error
		rollbackErr error
		pass        bool
		wantCommit  bool
		wantErrors  []error
	}{
		{name: "callback and rollback fail", callbackErr: callbackErr, rollbackErr: rollbackErr, wantErrors: []error{callbackErr, rollbackErr}},
		{name: "commit and rollback fail", commitErr: commitErr, rollbackErr: rollbackErr, wantCommit: true, wantErrors: []error{commitErr, rollbackErr}},
		{name: "rollback alone fails", rollbackErr: rollbackErr, wantCommit: true, wantErrors: []error{rollbackErr}},
		{name: "successful rollback", wantCommit: true},
		{name: "closed rollback", rollbackErr: closedErr, wantCommit: true},
		{name: "callback with successful rollback", callbackErr: callbackErr, wantErrors: []error{callbackErr}},
		{name: "callback with closed rollback", callbackErr: callbackErr, rollbackErr: closedErr, wantErrors: []error{callbackErr}},
		{name: "commit with successful rollback", commitErr: commitErr, wantCommit: true, wantErrors: []error{commitErr}},
		{name: "commit with closed rollback", commitErr: commitErr, rollbackErr: closedErr, wantCommit: true, wantErrors: []error{commitErr}},
		{name: "matcher passes callback and rollback fails", callbackErr: callbackErr, rollbackErr: rollbackErr, pass: true, wantCommit: true, wantErrors: []error{callbackErr, rollbackErr}},
		{name: "matcher passes callback with closed rollback", callbackErr: callbackErr, rollbackErr: closedErr, pass: true, wantCommit: true, wantErrors: []error{callbackErr}},
		{name: "matcher passes callback with successful rollback", callbackErr: callbackErr, pass: true, wantCommit: true, wantErrors: []error{callbackErr}},
		{name: "matcher passes callback but commit and rollback fail", callbackErr: callbackErr, commitErr: commitErr, rollbackErr: rollbackErr, pass: true, wantCommit: true, wantErrors: []error{commitErr, rollbackErr}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			parent := NewMockTx(t)
			nested := NewMockTx(t)
			ctx := pgcontext.With(context.Background(), pgcontext.WithTransaction(parent),
				pgcontext.WithFnTxPassMatcher(func(matcherCtx context.Context, err error) bool {
					require.Equal(t, parent, transactionFrom(t, matcherCtx))
					require.ErrorIs(t, err, tt.callbackErr)
					return tt.pass
				}))
			parent.EXPECT().Begin(ctx).Return(nested, nil).Once()
			if tt.wantCommit {
				nested.EXPECT().Commit(ctx).Return(tt.commitErr).Once()
			}
			nested.EXPECT().Rollback(ctx).Return(tt.rollbackErr).Once()

			err := New(nil).Transactional(ctx, func(callbackCtx context.Context) error {
				require.Equal(t, nested, transactionFrom(t, callbackCtx))
				return tt.callbackErr
			})
			if len(tt.wantErrors) == 0 {
				require.NoError(t, err)
				return
			}
			for _, expected := range tt.wantErrors {
				require.ErrorIs(t, err, expected)
			}
			var cause *pgconn.PgError
			require.True(t, errors.As(err, &cause))
			require.Same(t, tt.wantErrors[0], cause)
			if errors.Is(tt.rollbackErr, rollbackErr) {
				require.Contains(t, err.Error(), "can't rollback nested transaction")
			}
			if tt.commitErr != nil {
				require.Contains(t, err.Error(), "can't commit nested transaction")
			}
			if errors.Is(tt.rollbackErr, pgx.ErrTxClosed) {
				require.NotErrorIs(t, err, pgx.ErrTxClosed)
			}
		})
	}
}

func transactionFrom(t *testing.T, ctx context.Context) pgx.Tx {
	t.Helper()
	tx, ok := pgcontext.TransactionFrom(ctx)
	require.True(t, ok)
	return tx
}
