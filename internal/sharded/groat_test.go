package sharded

import (
	"context"
	"errors"
	"testing"

	"github.com/godepo/elephant/internal/pkg/pgcontext"
	"github.com/godepo/groat"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type Result struct {
	Tx    pgx.Tx
	Error error
	Rows  pgx.Rows
	Row   pgx.Row
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
	Expect      Expect
	Result      Result
}
type Deps struct {
	ctx        context.Context
	shardMocks []*MockPool
	faker      faker.Faker
	shardID    uint
}

type testCase = *groat.Case[Deps, State, *Hive]

func newTestCase(t *testing.T) testCase {
	tc := groat.New[Deps, State, *Hive](
		t,
		func(t *testing.T, deps Deps) *Hive {
			return &Hive{
				shards: []Pool{
					deps.shardMocks[0],
					deps.shardMocks[1],
					deps.shardMocks[2],
				},
				shardPicker: func(ctx context.Context, key string) uint {
					return deps.shardID
				},
			}
		},
		func(t *testing.T, deps Deps) Deps {
			deps.ctx = context.Background()
			deps.faker = faker.New()
			for range 3 {
				deps.shardMocks = append(deps.shardMocks, NewMockPool(t))
			}
			deps.shardID = deps.faker.UIntBetween(0, 2)
			return deps
		},
	)

	tc.Given(func(t *testing.T, state State) State {
		state.shardID = tc.Deps.shardID
		state.shardingKey = tc.Deps.faker.RandomStringWithLength(15)
		state.Faker = faker.New()
		return state
	})

	tc.Go()
	return tc
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

func ArrangeExpectError(t *testing.T, state State) State {
	t.Helper()
	state.Expect.Error = errors.New(state.Faker.RandomStringWithLength(10))
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

func ActBeginFailed(t *testing.T, deps Deps, state State) State {
	t.Helper()
	deps.shardMocks[state.shardID].EXPECT().Begin(state.ctx).Return(nil, state.Expect.Error)
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

func ActBeginTxFailed(t *testing.T, deps Deps, state State) State {
	t.Helper()
	deps.shardMocks[state.shardID].EXPECT().
		BeginTx(state.ctx, state.Expect.TxOptions).
		Return(nil, state.Expect.Error)
	return state
}

func ActQuery(t *testing.T, deps Deps, state State) State {
	t.Helper()
	deps.shardMocks[state.shardID].EXPECT().
		Query(state.ctx, state.Expect.Query, state.Expect.Args).
		Return(state.Expect.Rows, nil)
	return state
}

func ActQueryFailed(t *testing.T, deps Deps, state State) State {
	t.Helper()
	deps.shardMocks[state.shardID].EXPECT().
		Query(state.ctx, state.Expect.Query, state.Expect.Args).
		Return(nil, state.Expect.Error)
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

func ActExecFailed(t *testing.T, deps Deps, state State) State {
	t.Helper()
	deps.shardMocks[state.shardID].EXPECT().
		Exec(state.ctx, state.Expect.Query, state.Expect.Args).
		Return(pgconn.CommandTag{}, state.Expect.Error)
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

func AssertTxIsNil(t *testing.T, state State) {
	t.Helper()
	assert.Nil(t, state.Result.Tx)
}

func AssertErrorAs(err error) func(t *testing.T, state State) {
	return func(t *testing.T, state State) {
		t.Helper()
		assert.Equal(t, err, state.Result.Error)
	}
}

func AssertExpectedError(t *testing.T, state State) {
	t.Helper()
	require.Error(t, state.Result.Error)
	assert.ErrorIs(t, state.Result.Error, state.Expect.Error)
}

func AssertRows(t *testing.T, state State) {
	t.Helper()
	assert.Equal(t, state.Expect.Rows, state.Result.Rows)
}

func AssertRow(t *testing.T, state State) {
	t.Helper()
	assert.Equal(t, state.Expect.Row, state.Result.Row)
}
