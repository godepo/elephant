package shardedpg

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	t.Run("should be able to error if testPoolSize is 0", func(t *testing.T) {
		pool, err := New(0).Go()
		assert.Nil(t, pool)
		assert.ErrorIs(t, err, ErrWrongShardsPoolSize)
	})
	t.Run("should be able to error if nil shard picker provided", func(t *testing.T) {
		tc := newTestCase(t)
		tc.
			Given(
				ArrangeNilShardPicker,
			).
			Then(AssertErrorAs(ErrNoShardPickerProvided))
		tc.State.Result.ShardedPool, tc.State.Result.Error = tc.SUT.Picker(tc.State.shardPicker).Go()
	})
	t.Run("should be able to error if nil shard picker provided", func(t *testing.T) {
		tc := newTestCase(t)
		tc.
			Given(
				ArrangeNilValueShardPicker,
			).
			Then(AssertErrorAs(ErrNoShardPickerProvided))
		tc.State.Result.ShardedPool, tc.State.Result.Error = tc.SUT.Picker(tc.State.shardPicker).Go()
	})
	t.Run("should be able to error if no shard picker provided", func(t *testing.T) {
		tc := newTestCase(t)
		tc.
			Then(AssertErrorAs(ErrNoShardPickerProvided))
		tc.State.Result.ShardedPool, tc.State.Result.Error = tc.SUT.Picker(tc.State.shardPicker).Go()
	})
	t.Run("should be able to error if one of shards not provided", func(t *testing.T) {
		tc := newTestCase(t)
		tc.
			Given(
				ArrangeShardPicker,
			).
			Then(
				AssertErrorAs(ErrNotEnoughShardsProvided),
				AssertNilShardedPool,
			)
		tc.State.Result.ShardedPool, tc.State.Result.Error =
			tc.SUT.
				Shard(0, tc.State.shards[0]).
				Shard(1, tc.State.shards[1]).
				Picker(tc.State.shardPicker).
				Go()
	})
	t.Run("should be able to error if one of shards is nil", func(t *testing.T) {
		tc := newTestCase(t)
		tc.
			Given(
				ArrangeShardPicker,
				ArrangeNilShard(2),
			).
			Then(
				AssertErrorAs(ErrNilShardProvided),
				AssertNilShardedPool,
			)
		tc.State.Result.ShardedPool, tc.State.Result.Error =
			tc.SUT.
				Shard(0, tc.State.shards[0]).
				Shard(1, tc.State.shards[1]).
				Shard(2, tc.State.shards[2]).
				Picker(tc.State.shardPicker).
				Go()
	})
	t.Run("should be able to error if one of shards is nil val", func(t *testing.T) {
		tc := newTestCase(t)
		tc.
			Given(
				ArrangeShardPicker,
				ArrangeNilValueShard(0),
			).
			Then(
				AssertErrorAs(ErrNilShardProvided),
				AssertNilShardedPool,
			)
		tc.State.Result.ShardedPool, tc.State.Result.Error =
			tc.SUT.
				Shard(0, tc.State.shards[0]).
				Shard(1, tc.State.shards[1]).
				Shard(2, tc.State.shards[2]).
				Picker(tc.State.shardPicker).
				Go()
	})
	t.Run("should be able to error if one of shards not provided", func(t *testing.T) {
		tc := newTestCase(t)
		tc.
			Given(
				ArrangeShardPicker,
			).
			Then(
				AssertErrorAs(ErrNotEnoughShardsProvided),
				AssertNilShardedPool,
			)
		tc.State.Result.ShardedPool, tc.State.Result.Error =
			tc.SUT.
				Picker(tc.State.shardPicker).
				Go()
	})
	t.Run("should be able to query from shard by sharding key", func(t *testing.T) {
		tc := newTestCase(t)
		tc.
			Given(
				ArrangeContext, ExtendContextWithShardingKey,
				ArrangeQuery, ArrangeArgs, ArrangeRows,
				ArrangeShardPicker,
			).
			When(
				ActQuery,
			).
			Then(
				AssertNoError,
				AssertRows,
			)
		tc.State.Result.ShardedPool, tc.State.Result.Error =
			tc.SUT.
				Shard(0, tc.State.shards[0]).
				Shard(1, tc.State.shards[1]).
				Shard(2, tc.State.shards[2]).
				Picker(tc.State.shardPicker).
				Go()
		require.NoError(t, tc.State.Result.Error)
		tc.State.Result.Rows, tc.State.Result.Error =
			tc.State.Result.ShardedPool.Query(tc.State.ctx, tc.State.Expect.Query, tc.State.Expect.Args...)
	})
	t.Run("should be able to query from shard by sharding id in context", func(t *testing.T) {
		tc := newTestCase(t)
		tc.
			Given(
				ArrangeContext, ExtendContextWithShardID,
				ArrangeQuery, ArrangeArgs, ArrangeRows,
				ArrangeShardPicker,
			).
			When(ActQuery).
			Then(
				AssertNoError,
				AssertRows,
			)
		tc.State.Result.ShardedPool, tc.State.Result.Error =
			tc.SUT.
				Shard(0, tc.State.shards[0]).
				Shard(1, tc.State.shards[1]).
				Shard(2, tc.State.shards[2]).
				Picker(tc.State.shardPicker).
				Go()
		require.NoError(t, tc.State.Result.Error)
		tc.State.Result.Rows, tc.State.Result.Error =
			tc.State.Result.ShardedPool.Query(tc.State.ctx, tc.State.Expect.Query, tc.State.Expect.Args...)
	})
	t.Run("should be able to queryRow from shard by sharding key", func(t *testing.T) {
		tc := newTestCase(t)
		tc.
			Given(
				ArrangeContext, ExtendContextWithShardingKey,
				ArrangeQuery, ArrangeArgs, ArrangeRow,
				ArrangeShardPicker,
			).
			When(
				ActQueryRow,
			).
			Then(
				AssertNoError,
				AssertRow,
			)
		tc.State.Result.ShardedPool, tc.State.Result.Error =
			tc.SUT.
				Shard(0, tc.State.shards[0]).
				Shard(1, tc.State.shards[1]).
				Shard(2, tc.State.shards[2]).
				Picker(tc.State.shardPicker).
				Go()
		tc.State.Result.Row =
			tc.State.Result.ShardedPool.QueryRow(tc.State.ctx, tc.State.Expect.Query, tc.State.Expect.Args...)

	})
	t.Run("should be able to queryRow from shard by sharding id in context", func(t *testing.T) {
		tc := newTestCase(t)
		tc.
			Given(
				ArrangeContext, ExtendContextWithShardID,
				ArrangeQuery, ArrangeArgs, ArrangeRow,
				ArrangeShardPicker,
			).
			When(ActQueryRow).
			Then(
				AssertNoError,
				AssertRow,
			)
		tc.State.Result.ShardedPool, tc.State.Result.Error =
			tc.SUT.
				Shard(0, tc.State.shards[0]).
				Shard(1, tc.State.shards[1]).
				Shard(2, tc.State.shards[2]).
				Picker(tc.State.shardPicker).
				Go()
		require.NoError(t, tc.State.Result.Error)
		tc.State.Result.Row =
			tc.State.Result.ShardedPool.QueryRow(tc.State.ctx, tc.State.Expect.Query, tc.State.Expect.Args...)
	})
	t.Run("should be able to exec from shard by sharding key", func(t *testing.T) {
		tc := newTestCase(t)
		tc.
			Given(
				ArrangeContext, ExtendContextWithShardingKey,
				ArrangeQuery, ArrangeArgs,
				ArrangeShardPicker,
			).
			When(
				ActExec,
			).
			Then(
				AssertNoError,
			)
		tc.State.Result.ShardedPool, tc.State.Result.Error =
			tc.SUT.
				Shard(0, tc.State.shards[0]).
				Shard(1, tc.State.shards[1]).
				Shard(2, tc.State.shards[2]).
				Picker(tc.State.shardPicker).
				Go()
		require.NoError(t, tc.State.Result.Error)
		_, tc.State.Result.Error =
			tc.State.Result.ShardedPool.Exec(tc.State.ctx, tc.State.Expect.Query, tc.State.Expect.Args...)

	})
	t.Run("should be able to exec from shard by sharding id in context", func(t *testing.T) {
		tc := newTestCase(t)
		tc.
			Given(
				ArrangeContext, ExtendContextWithShardID,
				ArrangeQuery, ArrangeArgs,
				ArrangeShardPicker,
			).
			When(ActExec).
			Then(
				AssertNoError,
				AssertRow,
			)
		tc.State.Result.ShardedPool, tc.State.Result.Error =
			tc.SUT.
				Shard(0, tc.State.shards[0]).
				Shard(1, tc.State.shards[1]).
				Shard(2, tc.State.shards[2]).
				Picker(tc.State.shardPicker).
				Go()
		require.NoError(t, tc.State.Result.Error)
		_, tc.State.Result.Error =
			tc.State.Result.ShardedPool.Exec(tc.State.ctx, tc.State.Expect.Query, tc.State.Expect.Args...)
	})
	t.Run("should be able to begin from shard by sharding key", func(t *testing.T) {
		tc := newTestCase(t)
		tc.
			Given(
				ArrangeContext, ExtendContextWithShardingKey,
				ArrangeTx,
				ArrangeShardPicker,
			).
			When(
				ActBegin,
			).
			Then(
				AssertNoError,
				AssertTxAsExpected,
			)
		tc.State.Result.ShardedPool, tc.State.Result.Error =
			tc.SUT.
				Shard(0, tc.State.shards[0]).
				Shard(1, tc.State.shards[1]).
				Shard(2, tc.State.shards[2]).
				Picker(tc.State.shardPicker).
				Go()
		require.NoError(t, tc.State.Result.Error)
		tc.State.Result.Tx, tc.State.Result.Error =
			tc.State.Result.ShardedPool.Begin(tc.State.ctx)

	})
	t.Run("should be able to begin from shard by sharding id in context", func(t *testing.T) {
		tc := newTestCase(t)
		tc.
			Given(
				ArrangeContext, ExtendContextWithShardID,
				ArrangeTx,
				ArrangeShardPicker,
			).
			When(ActBegin).
			Then(
				AssertNoError,
				AssertRow,
			)
		tc.State.Result.ShardedPool, tc.State.Result.Error =
			tc.SUT.
				Shard(0, tc.State.shards[0]).
				Shard(1, tc.State.shards[1]).
				Shard(2, tc.State.shards[2]).
				Picker(tc.State.shardPicker).
				Go()
		require.NoError(t, tc.State.Result.Error)
		tc.State.Result.Tx, tc.State.Result.Error =
			tc.State.Result.ShardedPool.Begin(tc.State.ctx)
	})
	t.Run("should be able to beginTx from shard by sharding key", func(t *testing.T) {
		tc := newTestCase(t)
		tc.
			Given(
				ArrangeContext, ExtendContextWithShardingKey,
				ArrangeTx, ArrangeTxOptions,
				ArrangeShardPicker,
			).
			When(
				ActBeginTx,
			).
			Then(
				AssertNoError,
				AssertTxAsExpected,
			)
		tc.State.Result.ShardedPool, tc.State.Result.Error =
			tc.SUT.
				Shard(0, tc.State.shards[0]).
				Shard(1, tc.State.shards[1]).
				Shard(2, tc.State.shards[2]).
				Picker(tc.State.shardPicker).
				Go()
		require.NoError(t, tc.State.Result.Error)
		tc.State.Result.Tx, tc.State.Result.Error =
			tc.State.Result.ShardedPool.BeginTx(tc.State.ctx, tc.State.Expect.TxOptions)

	})
	t.Run("should be able to beginTx from shard by sharding id in context", func(t *testing.T) {
		tc := newTestCase(t)
		tc.
			Given(
				ArrangeContext, ExtendContextWithShardID,
				ArrangeTx, ArrangeTxOptions,
				ArrangeShardPicker,
			).
			When(ActBeginTx).
			Then(
				AssertNoError,
				AssertRow,
			)
		tc.State.Result.ShardedPool, tc.State.Result.Error =
			tc.SUT.
				Shard(0, tc.State.shards[0]).
				Shard(1, tc.State.shards[1]).
				Shard(2, tc.State.shards[2]).
				Picker(tc.State.shardPicker).
				Go()
		require.NoError(t, tc.State.Result.Error)
		tc.State.Result.Tx, tc.State.Result.Error =
			tc.State.Result.ShardedPool.BeginTx(tc.State.ctx, tc.State.Expect.TxOptions)
	})
	t.Run("should be able to Transactional from shard by sharding key", func(t *testing.T) {
		tc := newTestCase(t)
		tc.
			Given(
				ArrangeContext, ExtendContextWithShardingKey,
				ArrangeShardPicker,
			).
			When(
				ActTransactional,
			).
			Then(
				AssertNoError,
			)
		tc.State.Result.ShardedPool, tc.State.Result.Error =
			tc.SUT.
				Shard(0, tc.State.shards[0]).
				Shard(1, tc.State.shards[1]).
				Shard(2, tc.State.shards[2]).
				Picker(tc.State.shardPicker).
				Go()
		require.NoError(t, tc.State.Result.Error)
		tc.State.Result.Error =
			tc.State.Result.ShardedPool.Transactional(tc.State.ctx, func(ctx context.Context) error {
				return nil
			})

	})
	t.Run("should be able to Transactional from shard by sharding id in context", func(t *testing.T) {
		tc := newTestCase(t)
		tc.
			Given(
				ArrangeContext, ExtendContextWithShardID,
				ArrangeShardPicker,
			).
			When(ActTransactional).
			Then(
				AssertNoError,
			)
		tc.State.Result.ShardedPool, tc.State.Result.Error =
			tc.SUT.
				Shard(0, tc.State.shards[0]).
				Shard(1, tc.State.shards[1]).
				Shard(2, tc.State.shards[2]).
				Picker(tc.State.shardPicker).
				Go()
		require.NoError(t, tc.State.Result.Error)
		tc.State.Result.Error =
			tc.State.Result.ShardedPool.Transactional(tc.State.ctx, func(ctx context.Context) error {
				return nil
			})
	})
}
