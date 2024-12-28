package pgcontext

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jaswdr/faker/v2"
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

func TestShardIDFrom(t *testing.T) {
	t.Run("should be able return false if no shard id in context", func(t *testing.T) {
		id, ok := ShardIDFrom(context.Background())
		assert.False(t, ok)
		assert.Zero(t, id)
	})
	t.Run("should be able to set in context and read from it", func(t *testing.T) {
		shardID := faker.New().UInt()
		ctx := With(context.Background(), WithShardID(shardID))
		shard, ok := ShardIDFrom(ctx)
		require.True(t, ok)
		assert.Equal(t, shardID, shard)
	})
}

func TestShardingKeyFrom(t *testing.T) {
	t.Run("should be able return false if no shard key in context", func(t *testing.T) {
		key, ok := ShardingKeyFrom(context.Background())
		assert.Empty(t, key)
		assert.False(t, ok)
	})
	t.Run("should be able to set in context and read from it", func(t *testing.T) {
		shardingKey := faker.New().RandomStringWithLength(10)

		ctx := With(context.Background(), WithShardingKey(shardingKey))
		sharding, ok := ShardingKeyFrom(ctx)
		require.True(t, ok)
		assert.Equal(t, shardingKey, sharding)
	})
}

func TestWithMetricsLabel(t *testing.T) {
	t.Run("should be able return false, at empty context", func(t *testing.T) {
		key, ok := MetricsLabelsFrom(context.Background())
		assert.False(t, ok)
		assert.Empty(t, key)
	})
	t.Run("should be able to return metrics label and true when its in context", func(t *testing.T) {
		labels := []string{uuid.NewString()}
		ctx := With(context.Background(), WithMetricsLabel(labels...))
		out, ok := MetricsLabelsFrom(ctx)
		assert.True(t, ok)
		assert.Equal(t, labels, out)
	})
}

func TestWithTimeout(t *testing.T) {
	t.Run("should be able return false, at empty context", func(t *testing.T) {
		key, ok := QueryTimeoutFrom(context.Background())
		assert.False(t, ok)
		assert.Zero(t, key)
	})

	t.Run("should be able to return metrics label and true when its in context", func(t *testing.T) {
		ctx := With(context.Background(), WithTimeout(time.Hour))
		out, ok := QueryTimeoutFrom(ctx)
		assert.True(t, ok)
		assert.Equal(t, time.Hour, out)
	})
}
