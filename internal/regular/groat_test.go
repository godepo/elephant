package regular

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

func ArrangeContext(t *testing.T, state State) State {
	t.Helper()
	state.ctx = context.Background()
	return state
}

func ArrangeRecord(t *testing.T, state State) State {
	t.Helper()
	state.Record = Record{
		ID:    uuid.New(),
		Value: uuid.NewString(),
	}
	return state
}

func ArrangeExpectedError(t *testing.T, state State) State {
	t.Helper()
	state.ExpectError = errors.New(state.Faker.RandomStringWithLength(10))
	return state
}

func ArrangeTxMockInContext(t *testing.T, state State) State {
	state.TxMock = NewMockTx(t)
	state.ctx = pgcontext.With(state.ctx, pgcontext.WithTransaction(state.TxMock))
	return state
}

func ArrangeNestedTx(t *testing.T, state State) State {
	state.NestedTxMock = NewMockTx(t)
	return state
}

func ArrangeAsExpectError(err error) groat.Given[State] {
	return func(t *testing.T, state State) State {
		state.ExpectError = err
		return state
	}
}

func ArrangeTxOptions(_ *testing.T, state State) State {
	state.ctx = pgcontext.With(state.ctx, pgcontext.WithTxOptions(pgx.TxOptions{IsoLevel: pgx.Serializable}))
	return state
}

func ArrangeTxErrPassMatcher(fn pgcontext.TxPassMatcher) groat.Given[State] {
	return func(t *testing.T, state State) State {
		state.ctx = pgcontext.With(state.ctx, pgcontext.WithFnTxPassMatcher(fn))
		return state
	}
}

func InjectPoolMock(sut *Instance) groat.When[Deps, State] {
	return func(t *testing.T, deps Deps, state State) State {
		t.Helper()
		sut.db = deps.MockPool
		return state
	}
}

func ActBegin(sut *Instance) groat.When[Deps, State] {
	return func(t *testing.T, deps Deps, state State) State {
		tx, err := sut.Begin(state.ctx)
		require.NoError(t, err)
		state.Tx = tx
		state.ctx = pgcontext.With(state.ctx, pgcontext.WithTransaction(tx))
		return state
	}
}

func ActBeginTransaction(sut *Instance) groat.When[Deps, State] {
	return func(t *testing.T, deps Deps, state State) State {
		tx, err := sut.BeginTx(state.ctx, pgx.TxOptions{})
		require.NoError(t, err)
		state.Tx = tx
		state.ctx = pgcontext.With(state.ctx, pgcontext.WithTransaction(tx))
		return state
	}
}

func ActInsertRecord(sut *Instance) groat.When[Deps, State] {
	return func(t *testing.T, deps Deps, state State) State {
		cmd, err := sut.Exec(
			state.ctx,
			"INSERT INTO regular.instance (id, value) VALUES ($1, $2)",
			state.Record.ID, state.Record.Value,
		)
		require.NoError(t, err)
		require.True(t, cmd.Insert())
		return state
	}
}

func ActQueryRecord(sut *Instance) groat.When[Deps, State] {
	return func(t *testing.T, deps Deps, state State) State {
		AssertRecord(t, state.ctx, state, sut)
		return state
	}
}

func ActBeginWithError(_ pgx.TxOptions) groat.When[Deps, State] {
	return func(t *testing.T, deps Deps, state State) State {
		deps.MockPool.EXPECT().Begin(mock.Anything).Return(nil, state.ExpectError)
		return state
	}

}

func ActBeginTxWithError(opts pgx.TxOptions) groat.When[Deps, State] {
	return func(t *testing.T, deps Deps, state State) State {
		deps.MockPool.EXPECT().BeginTx(mock.Anything, opts).Return(nil, state.ExpectError)
		return state
	}
}

func ExpectQueryError(query string, args ...any) groat.When[Deps, State] {
	return func(t *testing.T, deps Deps, state State) State {
		deps.MockPool.EXPECT().Query(mock.Anything, query, args...).Return(nil, state.ExpectError)
		return state
	}
}

func ExpectExecError(query string, args ...any) groat.When[Deps, State] {
	return func(t *testing.T, deps Deps, state State) State {
		deps.MockPool.EXPECT().Exec(mock.Anything, query, args...).Return(pgconn.CommandTag{}, state.ExpectError)
		return state
	}
}

func ActFailAtNestedCommit(t *testing.T, _ Deps, state State) State {
	t.Helper()
	state.NestedTxMock.EXPECT().Commit(mock.Anything).Return(state.ExpectError)
	state.NestedTxMock.EXPECT().Rollback(mock.Anything).Return(pgx.ErrTxClosed)
	return state
}

func ActFailRollbackAtNestedCommit(t *testing.T, _ Deps, state State) State {
	t.Helper()
	state.NestedTxMock.EXPECT().Commit(mock.Anything).Return(nil)
	state.NestedTxMock.EXPECT().Rollback(mock.Anything).Return(state.ExpectError)
	return state
}

func ActBeginNested(_ *testing.T, _ Deps, state State) State {
	state.TxMock.EXPECT().Begin(mock.Anything).Return(state.NestedTxMock, nil)
	return state
}

func ActCommit(t *testing.T, _ Deps, state State) State {
	require.NoError(t, state.Tx.Commit(context.Background()))
	return state
}

func AssertNoError(t *testing.T, state State) {
	require.NoError(t, state.Result.Error)
}

func AssertExpectError(t *testing.T, state State) {
	require.Error(t, state.ExpectError)
	assert.ErrorIs(t, state.Result.Error, state.ExpectError)
}

func AssertCommitTransaction(t *testing.T, state State) {
	t.Helper()
	err := state.Tx.Commit(state.ctx)
	require.NoError(t, err)
}

func AssertRecord(t *testing.T, ctx context.Context, state State, db DB) {
	row := db.QueryRow(ctx, `SELECT * FROM regular.instance WHERE id = $1`, state.Record.ID)
	var rec Record
	require.NoError(t, row.Scan(&rec.ID, &rec.Value))
	assert.Equal(t, state.Record, rec)
}

func AssertHasRecord(sut *Instance) groat.Then[State] {
	return func(t *testing.T, state State) {
		AssertRecord(t, context.Background(), state, sut)
	}
}

func AssertRecordQueried(sut *Instance) groat.Then[State] {
	return func(t *testing.T, state State) {
		rows, err := sut.Query(context.Background(), `SELECT * FROM regular.instance WHERE id = $1`, state.Record.ID)
		require.NoError(t, err)
		require.NoError(t, rows.Err())
		items, err := pgx.CollectRows(rows, pgx.RowToStructByName[Record])
		require.NoError(t, err)
		assert.Contains(t, items, state.Record)
	}
}
