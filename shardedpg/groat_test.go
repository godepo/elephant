package shardedpg

import (
	"context"
	"testing"

	"github.com/godepo/elephant/internal/pkg/pgcontext"
	"github.com/godepo/groat"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type Result struct {
	Tx          pgx.Tx
	Error       error
	Rows        pgx.Rows
	Row         pgx.Row
	ShardedPool Pool
}

type Expect struct {
	Tx        *MockTx
	Error     error
	TxOptions pgx.TxOptions
	Query     string
	Args      []any
	Rows      pgx.Rows
	Row       pgx.Row
}

type State struct {
	ctx         context.Context
	Faker       faker.Faker
	shardID     uint
	shardingKey string
	shards      []Pool
	shardPicker ShardPicker
	Expect      Expect
	Result      Result
}
type Deps struct {
	ctx        context.Context
	shardMocks []*MockPool
	faker      faker.Faker
	shardID    uint
}

type testCase = *groat.Case[Deps, State, Builder]

func newTestCase(t *testing.T) testCase {
	const testPoolSize = 3
	tc := groat.New[Deps, State, Builder](
		t,
		func(t *testing.T, deps Deps) Builder {
			return New(testPoolSize)
		},
		func(t *testing.T, deps Deps) Deps {
			deps.ctx = context.Background()
			deps.faker = faker.New()
			deps.shardID = deps.faker.UIntBetween(0, testPoolSize-1)
			deps.shardMocks = make([]*MockPool, 0, testPoolSize)
			for range testPoolSize {
				deps.shardMocks = append(deps.shardMocks, NewMockPool(t))
			}
			return deps
		},
	)

	tc.Given(func(t *testing.T, state State) State {
		state.shardID = tc.Deps.shardID
		state.shardingKey = tc.Deps.faker.RandomStringWithLength(15)
		state.shards = make([]Pool, testPoolSize)
		for i := range testPoolSize {
			state.shards[i] = tc.Deps.shardMocks[i]
		}

		state.Faker = faker.New()
		return state
	})

	tc.Go()
	return tc
}

func ArrangeNilShardPicker(t *testing.T, state State) State {
	t.Helper()
	state.shardPicker = nil
	return state
}
func ArrangeNilValueShardPicker(t *testing.T, state State) State {
	t.Helper()
	var picker ShardPicker
	state.shardPicker = picker
	return state
}

func ArrangeNilShard(shardID uint) func(t *testing.T, state State) State {
	return func(t *testing.T, state State) State {
		t.Helper()
		state.shards[shardID] = nil
		return state
	}
}
func ArrangeNilValueShard(shardID uint) func(t *testing.T, state State) State {
	return func(t *testing.T, state State) State {
		t.Helper()
		var emptyValueShard *MockPool
		state.shards[shardID] = emptyValueShard
		return state
	}
}

func ArrangeShardPicker(t *testing.T, state State) State {
	t.Helper()
	state.shardPicker = func(ctx context.Context, key string) uint {
		t.Logf("pick shard: %d", state.shardID)
		return state.shardID
	}
	return state
}

func ArrangeContext(t *testing.T, state State) State {
	t.Helper()
	state.ctx = context.Background()
	return state
}

func ExtendContextWithShardID(t *testing.T, state State) State {
	t.Helper()
	state.ctx = pgcontext.With(state.ctx, pgcontext.WithShardID(state.shardID))
	return state
}
func ExtendContextWithShardingKey(t *testing.T, state State) State {
	t.Helper()
	state.ctx = pgcontext.With(state.ctx, pgcontext.WithShardingKey(state.shardingKey))
	return state
}

func ArrangeArgs(t *testing.T, state State) State {
	t.Helper()
	state.Expect.Args = []any{uuid.NewString()}
	return state
}

func ArrangeQuery(t *testing.T, state State) State {
	t.Helper()
	state.Expect.Query = state.Faker.RandomStringWithLength(20)
	return state
}

func ArrangeTx(t *testing.T, state State) State {
	t.Helper()
	state.Expect.Tx = NewMockTx(t)
	return state
}

func ArrangeRows(t *testing.T, state State) State {
	t.Helper()
	state.Expect.Rows = NewMockRows(t)
	return state
}
func ArrangeRow(t *testing.T, state State) State {
	t.Helper()
	state.Expect.Row = NewMockRow(t)
	return state
}

func ArrangeTxOptions(t *testing.T, state State) State {
	t.Helper()
	levels := []pgx.TxIsoLevel{pgx.ReadCommitted, pgx.RepeatableRead, pgx.ReadUncommitted, pgx.Serializable}

	state.Expect.TxOptions = pgx.TxOptions{
		IsoLevel: levels[state.Faker.IntBetween(0, len(levels)-1)],
	}
	return state
}

func ActBegin(t *testing.T, deps Deps, state State) State {
	t.Helper()
	deps.shardMocks[state.shardID].EXPECT().Begin(state.ctx).Return(state.Expect.Tx, nil)
	return state
}

func ActBeginTx(t *testing.T, deps Deps, state State) State {
	t.Helper()
	deps.shardMocks[state.shardID].EXPECT().BeginTx(
		state.ctx,
		state.Expect.TxOptions,
	).Return(state.Expect.Tx, nil)
	return state
}

func ActQuery(t *testing.T, deps Deps, state State) State {
	t.Helper()
	t.Logf("query at shard: %d", state.shardID)
	deps.shardMocks[state.shardID].EXPECT().
		Query(state.ctx, state.Expect.Query, state.Expect.Args).
		Return(state.Expect.Rows, nil)
	return state
}

func ActQueryRow(t *testing.T, deps Deps, state State) State {
	t.Helper()
	deps.shardMocks[state.shardID].EXPECT().
		QueryRow(state.ctx, state.Expect.Query, state.Expect.Args).
		Return(state.Expect.Row)
	return state
}

func ActExec(t *testing.T, deps Deps, state State) State {
	t.Helper()
	deps.shardMocks[state.shardID].EXPECT().
		Exec(state.ctx, state.Expect.Query, state.Expect.Args).
		Return(pgconn.CommandTag{}, nil)
	return state
}

func ActTransactional(t *testing.T, deps Deps, state State) State {
	t.Helper()
	deps.shardMocks[state.shardID].EXPECT().
		Transactional(mock.Anything, mock.Anything).
		RunAndReturn(
			func(ctx context.Context, f func(context.Context) error) error {
				return f(ctx)
			},
		)
	return state
}

func AssertNoError(t *testing.T, state State) {
	t.Helper()
	assert.NoError(t, state.Result.Error)
}

func AssertTxAsExpected(t *testing.T, state State) {
	t.Helper()
	assert.Equal(t, state.Expect.Tx, state.Result.Tx)
}

func AssertErrorAs(err error) func(t *testing.T, state State) {
	return func(t *testing.T, state State) {
		t.Helper()
		assert.Equal(t, err, state.Result.Error)
	}
}

func AssertRows(t *testing.T, state State) {
	t.Helper()
	assert.Equal(t, state.Expect.Rows, state.Result.Rows)
}

func AssertRow(t *testing.T, state State) {
	t.Helper()
	assert.Equal(t, state.Expect.Row, state.Result.Row)
}
func AssertNilShardedPool(t *testing.T, state State) {
	t.Helper()
	assert.Nil(t, state.Result.ShardedPool)
}
