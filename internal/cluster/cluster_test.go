package cluster

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

type Deps struct {
	Leader  *MockPool
	Fellows []*MockPool
	Tx      *MockTx
	Faker   faker.Faker
	Rows    *MockRows
	Row     *MockRow
}

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
	Result Result
	Expect Expect
	ctx    context.Context
	Faker  faker.Faker
}

const (
	runAtTx           = -2
	runAtLeader       = -1
	runAtFellowFirst  = 0
	runAtFellowSecond = 1
)

type testCase = *groat.Case[Deps, State, *Cluster]

func newTestCase(t *testing.T) testCase {
	tc := groat.New[Deps, State, *Cluster](t, func(t *testing.T, deps Deps) *Cluster {
		fellows := make([]Pool, 0, len(deps.Fellows))
		for _, fellow := range deps.Fellows {
			fellows = append(fellows, fellow)
		}
		return New(deps.Leader, fellows, WithLoadBalancer(DefaultLoadBalancer()))
	}, func(t *testing.T, deps Deps) Deps {
		t.Helper()
		deps.Leader = NewMockPool(t)
		deps.Fellows = []*MockPool{NewMockPool(t), NewMockPool(t)}
		deps.Tx = NewMockTx(t)
		deps.Faker = faker.New()
		deps.Rows = NewMockRows(t)
		deps.Row = NewMockRow(t)
		return deps
	})

	tc.Given(func(t *testing.T, state State) State {
		state.ctx = context.Background()
		return state
	})
	tc.Go()
	tc.State.Faker = tc.Deps.Faker
	return tc
}

func TestCluster_Begin(t *testing.T) {
	t.Run("should be able to be able", func(t *testing.T) {
		tc := newTestCase(t)

		tc.Given(ArrangeTx).When(ActBegin(runAtLeader)).
			Then(AssertTx, AssertNoError)

		tc.State.Result.Tx, tc.State.Result.Error = tc.SUT.Begin(tc.State.ctx)
	})

	t.Run("should be able to be run nested transaction", func(t *testing.T) {
		tc := newTestCase(t)

		tc.Given(InjectTxToContext(tc.Deps.Tx), ArrangeTx).When(ActBegin(runAtTx)).
			Then(AssertTx, AssertNoError)

		tc.State.Result.Tx, tc.State.Result.Error = tc.SUT.Begin(tc.State.ctx)
	})

	t.Run("should be able to fail when leader returned error", func(t *testing.T) {
		tc := newTestCase(t)

		tc.Given(ArrangeExpectError).When(ActBeginFail(runAtLeader)).
			Then(AssertExpectedError, AssertNoTx)

		tc.State.Result.Tx, tc.State.Result.Error = tc.SUT.Begin(tc.State.ctx)
	})

	t.Run("should be able fail when transaction reject run nested", func(t *testing.T) {
		tc := newTestCase(t)

		tc.Given(InjectTxToContext(tc.Deps.Tx), ArrangeExpectError).When(ActBeginFail(runAtTx)).
			Then(AssertExpectedError, AssertNoTx)

		tc.State.Result.Tx, tc.State.Result.Error = tc.SUT.Begin(tc.State.ctx)
	})
}

func TestCluster_BeginTx(t *testing.T) {
	t.Run("should be able to be able", func(t *testing.T) {
		tc := newTestCase(t)

		tc.Given(ArrangeTxOptions).
			When(ActBeginTx(runAtLeader)).
			Then(AssertNoError, AssertTx)

		tc.State.Result.Tx, tc.State.Result.Error = tc.SUT.BeginTx(tc.State.ctx, tc.State.Expect.TxOptions)
	})

	t.Run("should be able to begin as nested tx", func(t *testing.T) {
		tc := newTestCase(t)

		tc.Given(InjectTxToContext(tc.Deps.Tx), ArrangeTxOptions).
			When(ActBeginTx(runAtTx)).
			Then(AssertNoError, AssertTx)

		tc.State.Result.Tx, tc.State.Result.Error = tc.SUT.BeginTx(tc.State.ctx, tc.State.Expect.TxOptions)
	})

	t.Run("should be able to return error from leader", func(t *testing.T) {
		tc := newTestCase(t)

		tc.Given(ArrangeTxOptions, ArrangeExpectError).
			When(ActBeginTxFailed(runAtLeader)).
			Then(AssertExpectedError, AssertNoTx)

		tc.State.Result.Tx, tc.State.Result.Error = tc.SUT.BeginTx(tc.State.ctx, tc.State.Expect.TxOptions)
	})

	t.Run("should be able fail when can't run nested tx", func(t *testing.T) {
		tc := newTestCase(t)

		tc.Given(InjectTxToContext(tc.Deps.Tx), ArrangeExpectError).
			When(ActBeginTxFailed(runAtTx)).
			Then(AssertExpectedError, AssertNoTx)

		tc.State.Result.Tx, tc.State.Result.Error = tc.SUT.BeginTx(tc.State.ctx, tc.State.Expect.TxOptions)
	})
}

func TestCluster_Query(t *testing.T) {
	t.Run("should be able to be able", func(t *testing.T) {
		tc := newTestCase(t)
		tc.Given(ArrangeQuery, ArrangeArgs).
			When(ActQuery(runAtFellowSecond)).Then(AssertNoError, AssertRows)
		tc.State.Result.Rows, tc.State.Result.Error = tc.SUT.
			Query(tc.State.ctx, tc.State.Expect.Query, tc.State.Expect.Args...)
	})

	t.Run("should be able return error when failed fellow node", func(t *testing.T) {
		tc := newTestCase(t)
		tc.Given(ArrangeExpectError, ArrangeQuery, ArrangeArgs).
			When(ActQueryFailed(runAtFellowSecond)).Then(AssertExpectedError)
		tc.State.Result.Rows, tc.State.Result.Error = tc.SUT.
			Query(tc.State.ctx, tc.State.Expect.Query, tc.State.Expect.Args...)
	})

	t.Run("should be able to run query at transaction object from context", func(t *testing.T) {
		tc := newTestCase(t)

		tc.Given(InjectTxToContext(tc.Deps.Tx), ArrangeQuery, ArrangeArgs).
			When(ActQueryAtTx).
			Then(AssertNoError, AssertRows)

		tc.State.Result.Rows, tc.State.Result.Error = tc.SUT.
			Query(tc.State.ctx, tc.State.Expect.Query, tc.State.Expect.Args...)
	})

	t.Run("should be able to run query at leader ", func(t *testing.T) {
		tc := newTestCase(t)

		tc.Given(InjectCanWrite, ArrangeQuery, ArrangeArgs).
			When(ActQuery(runAtLeader)).
			Then(AssertNoError, AssertRows)

		tc.State.Result.Rows, tc.State.Result.Error = tc.SUT.
			Query(tc.State.ctx, tc.State.Expect.Query, tc.State.Expect.Args...)
	})
}

func TestCluster_QueryRow(t *testing.T) {
	t.Run("should be able to be able", func(t *testing.T) {
		tc := newTestCase(t)
		tc.Given(ArrangeQuery, ArrangeArgs).
			When(ActQueryRow(runAtFellowSecond)).Then(AssertRow)
		tc.State.Result.Row = tc.SUT.
			QueryRow(tc.State.ctx, tc.State.Expect.Query, tc.State.Expect.Args...)
	})

	t.Run("should be able to run query at transaction object from context", func(t *testing.T) {
		tc := newTestCase(t)
		tc.Given(InjectTxToContext(tc.Deps.Tx), ArrangeQuery, ArrangeArgs).
			When(ActQueryRow(runAtTx)).Then(AssertRow)
		tc.State.Result.Row = tc.SUT.
			QueryRow(tc.State.ctx, tc.State.Expect.Query, tc.State.Expect.Args...)
	})

	t.Run("should be able to run query at leader", func(t *testing.T) {
		tc := newTestCase(t)
		tc.Given(InjectCanWrite, ArrangeQuery, ArrangeArgs).
			When(ActQueryRow(runAtLeader)).Then(AssertRow)
		tc.State.Result.Row = tc.SUT.
			QueryRow(tc.State.ctx, tc.State.Expect.Query, tc.State.Expect.Args...)
	})
}

func TestCluster_Exec(t *testing.T) {
	t.Run("should be able to running at fellow", func(t *testing.T) {
		t.Run("success", func(t *testing.T) {
			tc := newTestCase(t)
			tc.Given(ArrangeQuery, ArrangeArgs).
				When(ActExec(runAtFellowSecond)).Then(AssertNoError)
			_, tc.State.Result.Error = tc.SUT.Exec(tc.State.ctx, tc.State.Expect.Query, tc.State.Expect.Args...)
		})

		t.Run("fail when node return error", func(t *testing.T) {
			tc := newTestCase(t)
			tc.Given(ArrangeQuery, ArrangeArgs).
				When(ActExecFailed(runAtFellowSecond)).Then(AssertNoError)

			_, tc.State.Result.Error = tc.SUT.Exec(tc.State.ctx, tc.State.Expect.Query, tc.State.Expect.Args...)
		})
	})

	t.Run("should be able to running at fellow", func(t *testing.T) {
		t.Run("success", func(t *testing.T) {
			tc := newTestCase(t)
			tc.Given(InjectTxToContext(tc.Deps.Tx), ArrangeQuery, ArrangeArgs).
				When(ActExec(runAtTx)).Then(AssertNoError)
			_, tc.State.Result.Error = tc.SUT.Exec(tc.State.ctx, tc.State.Expect.Query, tc.State.Expect.Args...)
		})

		t.Run("fail when node return error", func(t *testing.T) {
			tc := newTestCase(t)
			tc.Given(InjectTxToContext(tc.Deps.Tx), ArrangeQuery, ArrangeArgs).
				When(ActExecFailed(runAtTx)).Then(AssertNoError)

			_, tc.State.Result.Error = tc.SUT.Exec(tc.State.ctx, tc.State.Expect.Query, tc.State.Expect.Args...)
		})
	})
}

func TestCluster_Transactional(t *testing.T) {
	t.Run("should be able  run transaction at fellow", func(t *testing.T) {
		tc := newTestCase(t)

		tc.When(ActTransactional(runAtFellowSecond)).Then(AssertNoError)

		tc.State.Result.Error = tc.SUT.Transactional(tc.State.ctx, func(ctx context.Context) error {
			return nil
		})
	})
	t.Run("should be able to run transaction at leader", func(t *testing.T) {
		tc := newTestCase(t)
		tc.Given(ArrangeCanWrite).
			When(ActTransactional(runAtLeader)).
			Then(AssertNoError)

		tc.State.Result.Error = tc.SUT.Transactional(tc.State.ctx, func(ctx context.Context) error {
			return nil
		})
	})
}

func ArrangeCanWrite(t *testing.T, state State) State {
	t.Helper()
	state.ctx = pgcontext.With(state.ctx, pgcontext.WithCanWrite)
	return state
}

func ActTransactional(fellowNum int) groat.When[Deps, State] {
	return func(t *testing.T, deps Deps, state State) State {

		switch fellowNum {
		case runAtLeader:
			deps.Leader.EXPECT().
				Transactional(mock.Anything, mock.Anything).
				RunAndReturn(
					func(ctx context.Context, f func(context.Context) error) error {
						return f(ctx)
					})
		default:
			deps.Fellows[fellowNum].EXPECT().
				Transactional(mock.Anything, mock.Anything).
				RunAndReturn(
					func(ctx context.Context, f func(context.Context) error) error {
						return f(ctx)
					})
		}

		return state
	}
}

func ActExec(fellowNum int) groat.When[Deps, State] {
	return func(t *testing.T, deps Deps, state State) State {
		switch fellowNum {
		case runAtTx:
			deps.Tx.EXPECT().
				Exec(state.ctx, state.Expect.Query, state.Expect.Args...).Return(pgconn.CommandTag{}, nil)
		case runAtLeader:
			deps.Fellows[fellowNum].EXPECT().
				Exec(state.ctx, state.Expect.Query, state.Expect.Args...).Return(pgconn.CommandTag{}, nil)
		default:
			deps.Fellows[fellowNum].EXPECT().
				Exec(state.ctx, state.Expect.Query, state.Expect.Args...).Return(pgconn.CommandTag{}, nil)
		}

		return state
	}
}

func ActExecFailed(fellowNum int) groat.When[Deps, State] {
	return func(t *testing.T, deps Deps, state State) State {
		switch fellowNum {
		case runAtTx:
			deps.Tx.EXPECT().
				Exec(state.ctx, state.Expect.Query, state.Expect.Args...).
				Return(pgconn.CommandTag{}, state.Expect.Error)
		case runAtLeader:
			deps.Fellows[fellowNum].EXPECT().
				Exec(state.ctx, state.Expect.Query, state.Expect.Args...).
				Return(pgconn.CommandTag{}, state.Expect.Error)
		default:
			deps.Fellows[fellowNum].EXPECT().
				Exec(state.ctx, state.Expect.Query, state.Expect.Args...).
				Return(pgconn.CommandTag{}, state.Expect.Error)
		}

		return state
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

func ActQueryRow(fellowNum int) groat.When[Deps, State] {
	return func(t *testing.T, deps Deps, state State) State {
		switch fellowNum {
		case runAtTx:
			deps.Tx.EXPECT().
				QueryRow(state.ctx, state.Expect.Query, state.Expect.Args...).Return(deps.Row)
		case runAtLeader:
			deps.Leader.EXPECT().
				QueryRow(state.ctx, state.Expect.Query, state.Expect.Args...).Return(deps.Row)
		default:
			deps.Fellows[fellowNum].EXPECT().
				QueryRow(state.ctx, state.Expect.Query, state.Expect.Args...).Return(deps.Row)
		}
		state.Expect.Row = deps.Row
		return state
	}
}

func ActQuery(fellowNum int) groat.When[Deps, State] {
	return func(t *testing.T, deps Deps, state State) State {
		switch fellowNum {
		case runAtLeader:
			deps.Leader.EXPECT().
				Query(state.ctx, state.Expect.Query, state.Expect.Args...).Return(deps.Rows, nil)
		case runAtTx:
			deps.Tx.EXPECT().
				Query(state.ctx, state.Expect.Query, state.Expect.Args...).Return(deps.Rows, nil)
		default:
			deps.Fellows[fellowNum].EXPECT().
				Query(state.ctx, state.Expect.Query, state.Expect.Args...).Return(deps.Rows, nil)
		}
		state.Expect.Rows = deps.Rows
		return state
	}
}

func ActQueryAtTx(t *testing.T, deps Deps, state State) State {
	t.Helper()

	deps.Tx.EXPECT().
		Query(state.ctx, state.Expect.Query, state.Expect.Args...).Return(deps.Rows, nil)
	state.Expect.Rows = deps.Rows
	return state
}

func ActQueryFailed(fellowNum int) groat.When[Deps, State] {
	return func(t *testing.T, deps Deps, state State) State {
		deps.Fellows[fellowNum].EXPECT().
			Query(state.ctx, state.Expect.Query, state.Expect.Args...).Return(nil, state.Expect.Error)
		return state
	}
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

func ActBeginTx(fellowNum int) groat.When[Deps, State] {
	return func(t *testing.T, deps Deps, state State) State {
		t.Helper()
		switch fellowNum {
		case runAtTx:
			state.Expect.Tx = NewMockTx(t)
			deps.Tx.EXPECT().Begin(state.ctx).Return(state.Expect.Tx, nil)
		default:
			deps.Leader.EXPECT().BeginTx(state.ctx, state.Expect.TxOptions).Return(state.Expect.Tx, nil)
		}
		return state
	}
}

func ActBeginTxFailed(fellowNum int) groat.When[Deps, State] {
	return func(t *testing.T, deps Deps, state State) State {
		t.Helper()
		switch fellowNum {
		case runAtTx:
			deps.Tx.EXPECT().Begin(state.ctx).Return(nil, state.Expect.Error)
		default:
			deps.Leader.EXPECT().BeginTx(state.ctx, state.Expect.TxOptions).Return(nil, state.Expect.Error)
		}
		return state
	}
}

func ArrangeExpectError(t *testing.T, state State) State {
	t.Helper()
	state.Expect.Error = errors.New(state.Faker.RandomStringWithLength(10))
	return state
}

func ActBegin(fellowNum int) groat.When[Deps, State] {
	return func(t *testing.T, deps Deps, state State) State {
		switch fellowNum {
		case runAtTx:
			state.Expect.Tx = NewMockTx(t)
			deps.Tx.EXPECT().Begin(state.ctx).Return(state.Expect.Tx, nil)
		default:
			deps.Leader.EXPECT().Begin(state.ctx).Return(deps.Tx, nil)
		}
		return state
	}
}

func ActBeginFail(fellowNum int) groat.When[Deps, State] {
	return func(t *testing.T, deps Deps, state State) State {
		switch fellowNum {
		case runAtTx:
			deps.Tx.EXPECT().Begin(state.ctx).Return(nil, state.Expect.Error)
		default:
			deps.Leader.EXPECT().Begin(state.ctx).Return(nil, state.Expect.Error)
		}

		return state
	}
}

func ArrangeTx(t *testing.T, state State) State {
	t.Helper()
	state.Expect.Tx = NewMockTx(t)
	return state
}

func AssertTx(t *testing.T, state State) {
	t.Helper()
	assert.Equal(t, state.Expect.Tx, state.Result.Tx)
}

func AssertNoError(t *testing.T, state State) {
	t.Helper()
	require.NoError(t, state.Result.Error)
}

func AssertNoTx(t *testing.T, state State) {
	t.Helper()
	assert.Nil(t, state.Expect.Tx)
}

func AssertExpectedError(t *testing.T, state State) {
	t.Helper()
	require.Error(t, state.Result.Error)
	assert.ErrorIs(t, state.Result.Error, state.Expect.Error)
}

func ArrangeTxOptions(t *testing.T, state State) State {
	t.Helper()
	levels := []pgx.TxIsoLevel{pgx.ReadCommitted, pgx.RepeatableRead, pgx.ReadUncommitted, pgx.Serializable}

	state.Expect.TxOptions = pgx.TxOptions{
		IsoLevel: levels[state.Faker.IntBetween(0, len(levels)-1)],
	}
	return state
}

func InjectTxToContext(tx pgx.Tx) func(t *testing.T, state State) State {
	return func(t *testing.T, state State) State {
		state.ctx = pgcontext.With(state.ctx, pgcontext.WithTransaction(tx))
		return state
	}
}

func InjectCanWrite(t *testing.T, state State) State {
	t.Helper()

	state.ctx = pgcontext.With(state.ctx, pgcontext.WithCanWrite)
	return state
}
