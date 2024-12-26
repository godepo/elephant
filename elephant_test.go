package elephant

import (
	"context"
	"github.com/jaswdr/faker/v2"
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

func TestWithShardID(t *testing.T) {
	t.Run("should be able return zero id and false at empty context", func(t *testing.T) {
		id, ok := pgcontext.ShardIDFrom(context.Background())
		assert.False(t, ok)
		assert.Zero(t, id)
	})
	t.Run("should be able to return shard id and true when shardID in context", func(t *testing.T) {
		expectedShardID := faker.New().UInt()
		ctx := With(context.Background(), WithShardID(expectedShardID))
		id, ok := pgcontext.ShardIDFrom(ctx)
		assert.True(t, ok)
		assert.Equal(t, expectedShardID, id)
	})
}

func TestWithShardingKey(t *testing.T) {
	t.Run("should be able to return empty key and false at empty context", func(t *testing.T) {
		key, ok := pgcontext.ShardingKeyFrom(context.Background())
		assert.False(t, ok)
		assert.Empty(t, key)
	})
	t.Run("should be able to return shardingKey and true when its in context", func(t *testing.T) {
		expectedKey := faker.New().RandomStringWithLength(10)
		ctx := With(context.Background(), WithShardingKey(expectedKey))
		key, ok := pgcontext.ShardingKeyFrom(ctx)
		assert.True(t, ok)
		assert.Equal(t, expectedKey, key)
	})
}
