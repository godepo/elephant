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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	runAtTx           = -2
	runAtLeader       = -1
	runAtFellowFirst  = 0
	runAtFellowSecond = 1
)

func ArrangeCanWrite(t *testing.T, state State) State {
	t.Helper()
	state.ctx = pgcontext.With(state.ctx, pgcontext.WithCanWrite)
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
				Exec(state.ctx, state.Expect.Query, state.Expect.Args).Return(pgconn.CommandTag{}, nil)
		case runAtLeader:
			deps.Fellows[fellowNum].EXPECT().
				Exec(state.ctx, state.Expect.Query, state.Expect.Args).Return(pgconn.CommandTag{}, nil)
		default:
			deps.Fellows[fellowNum].EXPECT().
				Exec(state.ctx, state.Expect.Query, state.Expect.Args).Return(pgconn.CommandTag{}, nil)
		}

		return state
	}
}

func ActExecFailed(fellowNum int) groat.When[Deps, State] {
	return func(t *testing.T, deps Deps, state State) State {
		switch fellowNum {
		case runAtTx:
			deps.Tx.EXPECT().
				Exec(state.ctx, state.Expect.Query, state.Expect.Args).
				Return(pgconn.CommandTag{}, state.Expect.Error)
		case runAtLeader:
			deps.Fellows[fellowNum].EXPECT().
				Exec(state.ctx, state.Expect.Query, state.Expect.Args).
				Return(pgconn.CommandTag{}, state.Expect.Error)
		default:
			deps.Fellows[fellowNum].EXPECT().
				Exec(state.ctx, state.Expect.Query, state.Expect.Args).
				Return(pgconn.CommandTag{}, state.Expect.Error)
		}

		return state
	}
}

func ActQueryRow(fellowNum int) groat.When[Deps, State] {
	return func(t *testing.T, deps Deps, state State) State {
		switch fellowNum {
		case runAtTx:
			deps.Tx.EXPECT().
				QueryRow(state.ctx, state.Expect.Query, state.Expect.Args).Return(deps.Row)
		case runAtLeader:
			deps.Leader.EXPECT().
				QueryRow(state.ctx, state.Expect.Query, state.Expect.Args).Return(deps.Row)
		default:
			deps.Fellows[fellowNum].EXPECT().
				QueryRow(state.ctx, state.Expect.Query, state.Expect.Args).Return(deps.Row)
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
				Query(state.ctx, state.Expect.Query, state.Expect.Args).Return(deps.Rows, nil)
		case runAtTx:
			deps.Tx.EXPECT().
				Query(state.ctx, state.Expect.Query, state.Expect.Args).Return(deps.Rows, nil)
		default:
			deps.Fellows[fellowNum].EXPECT().
				Query(state.ctx, state.Expect.Query, state.Expect.Args).Return(deps.Rows, nil)
		}
		state.Expect.Rows = deps.Rows
		return state
	}
}

func ActQueryAtTx(t *testing.T, deps Deps, state State) State {
	t.Helper()

	deps.Tx.EXPECT().
		Query(state.ctx, state.Expect.Query, state.Expect.Args).Return(deps.Rows, nil)
	state.Expect.Rows = deps.Rows
	return state
}

func ActQueryFailed(fellowNum int) groat.When[Deps, State] {
	return func(t *testing.T, deps Deps, state State) State {
		deps.Fellows[fellowNum].EXPECT().
			Query(state.ctx, state.Expect.Query, state.Expect.Args).Return(nil, state.Expect.Error)
		return state
	}
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

func AssertRows(t *testing.T, state State) {
	t.Helper()
	assert.Equal(t, state.Expect.Rows, state.Result.Rows)
}

func AssertRow(t *testing.T, state State) {
	t.Helper()
	assert.Equal(t, state.Expect.Row, state.Result.Row)
}
