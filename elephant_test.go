package elephant

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/require"

	"github.com/godepo/elephant/internal/pkg/pgcontext"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
)

func TestCanWriteFrom(t *testing.T) {
	t.Run("should be able return false, at empty context", func(t *testing.T) {
		assert.False(t, pgcontext.CanWriteFrom(context.Background()))
	})
	t.Run("should be able to return true, when can write set in context", func(t *testing.T) {
		ctx := With(context.Background(), WithCanWrite)
		assert.True(t, pgcontext.CanWriteFrom(ctx))
		assert.True(t, CanWriteFrom(ctx))
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

		pubID, ok := ShardIDFrom(ctx)
		require.True(t, ok)
		assert.Equal(t, expectedShardID, pubID)
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

		pubKey, ok := ShardingKeyFrom(ctx)
		require.True(t, ok)
		assert.Equal(t, expectedKey, pubKey)
	})
}

func TestWithMetricsLabel(t *testing.T) {
	t.Run("should be able return false, at empty context", func(t *testing.T) {
		key, ok := pgcontext.MetricsLabelsFrom(context.Background())
		assert.False(t, ok)
		assert.Empty(t, key)
	})
	t.Run("should be able to return metrics label and true when its in context", func(t *testing.T) {
		labels := []string{uuid.NewString()}
		ctx := With(context.Background(), WithMetricsLabel(labels...))
		out, ok := pgcontext.MetricsLabelsFrom(ctx)
		assert.True(t, ok)
		assert.Equal(t, labels, out)

		pubLabels, ok := MetricsLabelFrom(ctx)
		require.True(t, ok)
		assert.Equal(t, labels, pubLabels)
	})
}

func TestWithTimeout(t *testing.T) {
	t.Run("should be able return false, at empty context", func(t *testing.T) {
		key, ok := pgcontext.QueryTimeoutFrom(context.Background())
		assert.False(t, ok)
		assert.Zero(t, key)
	})

	t.Run("should be able to return metrics label and true when its in context", func(t *testing.T) {
		ctx := With(context.Background(), WithTimeout(time.Hour))
		out, ok := pgcontext.QueryTimeoutFrom(ctx)
		assert.True(t, ok)
		assert.Equal(t, time.Hour, out)

		pubOut, ok := TimeoutFrom(ctx)
		assert.True(t, ok)
		assert.Equal(t, time.Hour, pubOut)
	})
}

func TestWithFnTxPassMatcher(t *testing.T) {
	t.Run("should be able return false, at empty context", func(t *testing.T) {
		_, ok := pgcontext.TxPassMatcherFrom(context.Background())
		assert.False(t, ok)
	})
	t.Run("should be able to return tx pass matcher, when its in context", func(t *testing.T) {
		ctx := With(context.Background(), WithFnTxPassMatcher(func(ctx context.Context, err error) bool {
			return true
		}))
		fn, ok := pgcontext.TxPassMatcherFrom(ctx)
		require.True(t, ok)
		assert.True(t, fn(ctx, errors.New(uuid.NewString())))
	})
}

func TestWithTxOptions(t *testing.T) {
	t.Run("should be able return false, at empty context", func(t *testing.T) {
		tx, ok := pgcontext.TxOptionsFrom(context.Background())
		assert.False(t, ok)
		assert.Zero(t, tx)
	})
	t.Run("should be able to return tx options, when its in context", func(t *testing.T) {
		exp := pgx.TxOptions{
			IsoLevel: pgx.Serializable,
		}
		ctx := With(context.Background(), WithTxOptions(exp))
		opt, ok := pgcontext.TxOptionsFrom(ctx)
		require.True(t, ok)
		assert.Equal(t, exp, opt)

		pubOpt, ok := TxOptionsFrom(ctx)
		require.True(t, ok)
		assert.Equal(t, exp, pubOpt)
	})
}
