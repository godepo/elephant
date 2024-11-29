package elephant

import (
	"context"
	"testing"

	"github.com/godepo/elephant/internal/pkg/pgcontext"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
)

func TestCanWriteFrom(t *testing.T) {
	t.Run("should be able return false, at empty context", func(t *testing.T) {
		assert.False(t, pgcontext.CanWriteFrom(context.Background()))
	})
	t.Run("should be able to return true, when can write set in context", func(t *testing.T) {
		assert.True(t, pgcontext.CanWriteFrom(With(context.Background(), WithCanWrite)))
	})
}

func TestWithTransaction(t *testing.T) {
	t.Run("should be able return false, at empty context", func(t *testing.T) {
		tx, ok := pgcontext.TransactionFrom(context.Background())
		assert.False(t, ok)
		assert.Nil(t, tx)
	})

	t.Run("should be able to write in context empty interface", func(t *testing.T) {
		var tx pgx.Tx

		ctx := With(context.Background(), WithTransaction(tx))

		tx, ok := pgcontext.TransactionFrom(ctx)
		assert.False(t, ok)
		assert.Nil(t, tx)
	})

	t.Run("should be able to write in context and get from it", func(t *testing.T) {

		ctx := With(context.Background(), WithTransaction(NewMockTx(t)))

		tx, ok := pgcontext.TransactionFrom(ctx)
		assert.True(t, ok)
		assert.NotNil(t, tx)
	})
}
