package pgcontext

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCanWriteFrom(t *testing.T) {
	t.Run("should be able return false, at empty context", func(t *testing.T) {
		assert.False(t, CanWriteFrom(context.Background()))
	})
	t.Run("should be able to return true, when can write set in context", func(t *testing.T) {
		assert.True(t, CanWriteFrom(With(context.Background(), WithCanWrite)))
	})
}

func TestTransactionFrom(t *testing.T) {
	t.Run("should be able return false, at empty context", func(t *testing.T) {
		tx, ok := TransactionFrom(context.Background())
		assert.False(t, ok)
		assert.Nil(t, tx)
	})

	t.Run("should be able to write in context empty interface", func(t *testing.T) {
		var tx pgx.Tx

		ctx := With(context.Background(), WithTransaction(tx))

		tx, ok := TransactionFrom(ctx)
		assert.False(t, ok)
		assert.Nil(t, tx)
	})

	t.Run("should be able to write in context and get from it", func(t *testing.T) {

		ctx := With(context.Background(), WithTransaction(NewMockTx(t)))

		tx, ok := TransactionFrom(ctx)
		assert.True(t, ok)
		assert.NotNil(t, tx)
	})
}

func TestTxOptionsFrom(t *testing.T) {
	t.Run("should be able return false, at empty context", func(t *testing.T) {
		_, ok := TxOptionsFrom(context.Background())
		assert.False(t, ok)
	})

	t.Run("should be able to set in context and read from it", func(t *testing.T) {
		expOpt := pgx.TxOptions{
			IsoLevel: pgx.Serializable,
		}
		ctx := With(context.Background(), WithTxOptions(expOpt))
		opt, ok := TxOptionsFrom(ctx)
		require.True(t, ok)
		assert.Equal(t, expOpt, opt)
	})
}

func TestTxPassMatcherFrom(t *testing.T) {
	t.Run("should be able return false, at empty context", func(t *testing.T) {
		_, ok := TxPassMatcherFrom(context.Background())
		assert.False(t, ok)
	})
	t.Run("should be able to set in context and read from it", func(t *testing.T) {
		ctx := With(context.Background(), WithFnTxPassMatcher(func(context.Context, error) bool {
			return true
		}))
		fn, ok := TxPassMatcherFrom(ctx)
		require.True(t, ok)
		assert.True(t, fn(ctx, errors.New(uuid.NewString())))

	})
}
