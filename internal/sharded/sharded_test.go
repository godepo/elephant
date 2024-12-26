package sharded

import (
	"context"
	"errors"
	"testing"

	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFailedRow(t *testing.T) {
	t.Run("should be able to scan err", func(t *testing.T) {
		expErr := errors.New(faker.New().RandomStringWithLength(10))
		row := failedRow{
			err: expErr,
		}
		err := row.Scan()
		assert.Equal(t, expErr, err)
	})
}

func TestNew(t *testing.T) {
	t.Run("should be able to be able", func(t *testing.T) {
		mockPool := []Pool{NewMockPool(t), NewMockPool(t), NewMockPool(t)}
		shardPicker := func(ctx context.Context, key string) uint {
			return 0
		}
		sharded := New(mockPool, shardPicker)
		require.NotNil(t, sharded)
		assert.Equal(t, mockPool, sharded.shards)
		assert.NotNil(t, sharded.shardPicker)
	})
}

func TestSharded_Begin(t *testing.T) {
	t.Run("should be able to return error if could not pick shard", func(t *testing.T) {
		tc := newTestCase(t)
		tc.
			Given(ArrangeContext).
			Then(AssertTxIsNil, AssertErrorAs(ErrCouldNotPickShard))

		tc.State.Result.Tx, tc.State.Result.Error = tc.SUT.Begin(tc.State.ctx)
	})
	t.Run("should be able to get shard by id and begin", func(t *testing.T) {
		tc := newTestCase(t)
		tc.
			Given(
				ArrangeContext, ExtendContextWithShardID,
				ArrangeTx,
			).
			When(ActBegin).
			Then(AssertTxAsExpected, AssertNoError)

		tc.State.Result.Tx, tc.State.Result.Error = tc.SUT.Begin(tc.State.ctx)
	})
	t.Run("should be able to get shard by id and begin and return error", func(t *testing.T) {
		tc := newTestCase(t)
		tc.
			Given(
				ArrangeContext, ExtendContextWithShardID,
				ArrangeExpectError,
			).
			When(ActBeginFailed).
			Then(AssertTxIsNil, AssertExpectedError)

		tc.State.Result.Tx, tc.State.Result.Error = tc.SUT.Begin(tc.State.ctx)
	})
	t.Run("should be able to get shard by key", func(t *testing.T) {
		tc := newTestCase(t)
		tc.
			Given(
				ArrangeContext, ExtendContextWithShardingKey,
				ArrangeTx,
			).
			When(ActBegin).
			Then(AssertTxAsExpected, AssertNoError)

		tc.State.Result.Tx, tc.State.Result.Error = tc.SUT.Begin(tc.State.ctx)
	})
	t.Run("should be able to get shard by key and return err", func(t *testing.T) {
		tc := newTestCase(t)
		tc.
			Given(
				ArrangeContext, ExtendContextWithShardingKey,
				ArrangeExpectError,
			).
			When(ActBeginFailed).
			Then(AssertTxIsNil, AssertExpectedError)

		tc.State.Result.Tx, tc.State.Result.Error = tc.SUT.Begin(tc.State.ctx)
	})
}

func TestSharded_BeginTx(t *testing.T) {
	t.Run("should be able to return error if could not pick shard", func(t *testing.T) {
		tc := newTestCase(t)
		tc.
			Given(ArrangeContext).
			Then(AssertTxIsNil, AssertErrorAs(ErrCouldNotPickShard))

		tc.State.Result.Tx, tc.State.Result.Error = tc.SUT.BeginTx(tc.State.ctx, tc.State.Expect.TxOptions)
	})
	t.Run("should be able to get shard by id and begin", func(t *testing.T) {
		tc := newTestCase(t)
		tc.
			Given(
				ArrangeContext, ExtendContextWithShardID,
				ArrangeTx, ArrangeTxOptions,
			).
			When(ActBeginTx).
			Then(AssertTxAsExpected, AssertNoError)

		tc.State.Result.Tx, tc.State.Result.Error = tc.SUT.BeginTx(tc.State.ctx, tc.State.Expect.TxOptions)
	})
	t.Run("should be able to get shard by id and begin and return error", func(t *testing.T) {
		tc := newTestCase(t)
		tc.
			Given(
				ArrangeContext, ExtendContextWithShardID,
				ArrangeTxOptions,
				ArrangeExpectError,
			).
			When(ActBeginTxFailed).
			Then(AssertTxIsNil, AssertExpectedError)

		tc.State.Result.Tx, tc.State.Result.Error = tc.SUT.BeginTx(tc.State.ctx, tc.State.Expect.TxOptions)
	})
	t.Run("should be able to get shard by key", func(t *testing.T) {
		tc := newTestCase(t)
		tc.
			Given(
				ArrangeContext, ExtendContextWithShardingKey,
				ArrangeTx, ArrangeTxOptions,
			).
			When(ActBeginTx).
			Then(AssertTxAsExpected, AssertNoError)

		tc.State.Result.Tx, tc.State.Result.Error = tc.SUT.BeginTx(tc.State.ctx, tc.State.Expect.TxOptions)
	})
	t.Run("should be able to get shard by key and return err", func(t *testing.T) {
		tc := newTestCase(t)
		tc.
			Given(
				ArrangeContext, ExtendContextWithShardingKey,
				ArrangeTxOptions,
				ArrangeExpectError,
			).
			When(ActBeginTxFailed).
			Then(AssertTxIsNil, AssertExpectedError)

		tc.State.Result.Tx, tc.State.Result.Error = tc.SUT.BeginTx(tc.State.ctx, tc.State.Expect.TxOptions)
	})
}

func TestSharded_Query(t *testing.T) {
	t.Run("should be able to return error if could not pick shard", func(t *testing.T) {
		tc := newTestCase(t)
		tc.
			Given(
				ArrangeContext,
				ArrangeQuery, ArrangeArgs,
			).
			Then(AssertErrorAs(ErrCouldNotPickShard))

		tc.State.Result.Rows, tc.State.Result.Error =
			tc.SUT.Query(
				tc.State.ctx,
				tc.State.Expect.Query,
				tc.State.Expect.Args...,
			)
	})
	t.Run("should be able to get shard by id", func(t *testing.T) {
		tc := newTestCase(t)
		tc.
			Given(
				ArrangeContext, ExtendContextWithShardID,
				ArrangeQuery, ArrangeArgs, ArrangeRows,
			).
			When(ActQuery).
			Then(AssertRows, AssertNoError)

		tc.State.Result.Rows, tc.State.Result.Error =
			tc.SUT.Query(
				tc.State.ctx,
				tc.State.Expect.Query,
				tc.State.Expect.Args...,
			)
	})
	t.Run("should be able to get shard by id and return error", func(t *testing.T) {
		tc := newTestCase(t)
		tc.
			Given(
				ArrangeContext, ExtendContextWithShardID,
				ArrangeQuery, ArrangeArgs,
				ArrangeExpectError,
			).
			When(ActQueryFailed).
			Then(AssertExpectedError)

		tc.State.Result.Rows, tc.State.Result.Error =
			tc.SUT.Query(
				tc.State.ctx,
				tc.State.Expect.Query,
				tc.State.Expect.Args...,
			)
	})
	t.Run("should be able to get shard by key", func(t *testing.T) {
		tc := newTestCase(t)
		tc.
			Given(
				ArrangeContext, ExtendContextWithShardingKey,
				ArrangeQuery, ArrangeArgs, ArrangeRows,
			).
			When(ActQuery).
			Then(AssertRows, AssertNoError)

		tc.State.Result.Rows, tc.State.Result.Error =
			tc.SUT.Query(
				tc.State.ctx,
				tc.State.Expect.Query,
				tc.State.Expect.Args...,
			)
	})
	t.Run("should be able to get shard by key and return err", func(t *testing.T) {
		tc := newTestCase(t)
		tc.
			Given(
				ArrangeContext, ExtendContextWithShardingKey,
				ArrangeQuery, ArrangeArgs,
				ArrangeExpectError,
			).
			When(ActQueryFailed).
			Then(AssertExpectedError)

		tc.State.Result.Rows, tc.State.Result.Error =
			tc.SUT.Query(
				tc.State.ctx,
				tc.State.Expect.Query,
				tc.State.Expect.Args...,
			)
	})
}

func TestSharded_QueryRow(t *testing.T) {
	t.Run("should be able to return error if could not pick shard", func(t *testing.T) {
		tc := newTestCase(t)
		tc.
			Given(
				ArrangeContext,
				ArrangeQuery, ArrangeArgs,
			).
			Then(AssertErrorAs(ErrCouldNotPickShard))

		tc.State.Result.Row =
			tc.SUT.QueryRow(
				tc.State.ctx,
				tc.State.Expect.Query,
				tc.State.Expect.Args...,
			)
		tc.State.Result.Error = tc.State.Result.Row.Scan()
	})
	t.Run("should be able to get shard by id", func(t *testing.T) {
		tc := newTestCase(t)
		tc.
			Given(
				ArrangeContext, ExtendContextWithShardID,
				ArrangeQuery, ArrangeArgs, ArrangeRow,
			).
			When(ActQueryRow).
			Then(AssertRow)

		tc.State.Result.Row =
			tc.SUT.QueryRow(
				tc.State.ctx,
				tc.State.Expect.Query,
				tc.State.Expect.Args...,
			)
	})
	t.Run("should be able to get shard by key", func(t *testing.T) {
		tc := newTestCase(t)
		tc.
			Given(
				ArrangeContext, ExtendContextWithShardingKey,
				ArrangeQuery, ArrangeArgs, ArrangeRow,
			).
			When(ActQueryRow).
			Then(AssertRow)

		tc.State.Result.Row =
			tc.SUT.QueryRow(
				tc.State.ctx,
				tc.State.Expect.Query,
				tc.State.Expect.Args...,
			)
	})
}

func TestSharded_Exec(t *testing.T) {
	t.Run("should be able to return error if could not pick shard", func(t *testing.T) {
		tc := newTestCase(t)
		tc.
			Given(
				ArrangeContext,
				ArrangeQuery, ArrangeArgs,
			).
			Then(AssertErrorAs(ErrCouldNotPickShard))

		_, tc.State.Result.Error =
			tc.SUT.Exec(
				tc.State.ctx,
				tc.State.Expect.Query,
				tc.State.Expect.Args...,
			)
	})
	t.Run("should be able to get shard by id", func(t *testing.T) {
		tc := newTestCase(t)
		tc.
			Given(
				ArrangeContext, ExtendContextWithShardID,
				ArrangeQuery, ArrangeArgs,
			).
			When(ActExec).
			Then(AssertNoError)

		_, tc.State.Result.Error =
			tc.SUT.Exec(
				tc.State.ctx,
				tc.State.Expect.Query,
				tc.State.Expect.Args...,
			)
	})
	t.Run("should be able to get shard by id and return error", func(t *testing.T) {
		tc := newTestCase(t)
		tc.
			Given(
				ArrangeContext, ExtendContextWithShardID,
				ArrangeQuery, ArrangeArgs,
				ArrangeExpectError,
			).
			When(ActExecFailed).
			Then(AssertExpectedError)

		_, tc.State.Result.Error =
			tc.SUT.Exec(
				tc.State.ctx,
				tc.State.Expect.Query,
				tc.State.Expect.Args...,
			)
	})
	t.Run("should be able to get shard by key", func(t *testing.T) {
		tc := newTestCase(t)
		tc.
			Given(
				ArrangeContext, ExtendContextWithShardingKey,
				ArrangeQuery, ArrangeArgs,
			).
			When(ActExec).
			Then(AssertNoError)

		_, tc.State.Result.Error =
			tc.SUT.Exec(
				tc.State.ctx,
				tc.State.Expect.Query,
				tc.State.Expect.Args...,
			)
	})
	t.Run("should be able to get shard by key and return err", func(t *testing.T) {

		tc := newTestCase(t)
		tc.
			Given(
				ArrangeContext, ExtendContextWithShardingKey,
				ArrangeQuery, ArrangeArgs,
				ArrangeExpectError,
			).
			When(ActExecFailed).
			Then(AssertExpectedError)

		_, tc.State.Result.Error =
			tc.SUT.Exec(
				tc.State.ctx,
				tc.State.Expect.Query,
				tc.State.Expect.Args...,
			)
	})
}

func TestSharded_Transactional(t *testing.T) {
	t.Run("should be able to return error if could not pick shard", func(t *testing.T) {
		tc := newTestCase(t)
		tc.
			Given(ArrangeContext).
			Then(AssertErrorAs(ErrCouldNotPickShard))

		tc.State.Result.Error =
			tc.SUT.Transactional(
				tc.State.ctx,
				func(ctx context.Context) error {
					return nil
				},
			)
	})
	t.Run("should be able to get shard by id", func(t *testing.T) {
		tc := newTestCase(t)
		tc.
			Given(
				ArrangeContext, ExtendContextWithShardID,
			).
			When(ActTransactional).
			Then(AssertNoError)

		tc.State.Result.Error =
			tc.SUT.Transactional(
				tc.State.ctx,
				func(ctx context.Context) error {
					return nil
				},
			)
	})

	t.Run("should be able to get shard by key", func(t *testing.T) {
		tc := newTestCase(t)
		tc.
			Given(
				ArrangeContext, ExtendContextWithShardingKey,
			).
			When(ActTransactional).
			Then(AssertNoError)

		tc.State.Result.Error =
			tc.SUT.Transactional(
				tc.State.ctx,
				func(ctx context.Context) error {
					return nil
				},
			)
	})
}
