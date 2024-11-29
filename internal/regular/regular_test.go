package regular

import (
	"context"
	"errors"
	"testing"

	"github.com/godepo/elephant/internal/pkg/pgcontext"
	"github.com/godepo/groat"
	"github.com/godepo/groat/integration"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type Deps struct {
	DB       *pgxpool.Pool `groat:"pgxpool"`
	Faker    faker.Faker
	MockDB   *MockDB
	MockRows *MockRows
	MockPool *MockPool
}

type Record struct {
	ID    uuid.UUID `db:"id"`
	Value string    `db:"value"`
}

type Result struct {
	Error error
}

type State struct {
	Record       Record
	Faker        faker.Faker
	ctx          context.Context
	Tx           pgx.Tx
	ExpectError  error
	Result       Result
	TxMock       *MockTx
	NestedTxMock *MockTx
}

var suite *integration.Container[Deps, State, *Instance]

func TestNew(t *testing.T) {
	tcs := suite.Case(t)
	tcs.Go()

	require.NotNil(t, tcs.SUT)
	require.NotNil(t, tcs.SUT.db)
}

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
		assertRecord(t, state.ctx, state, sut)
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

func AssertExpectError(t *testing.T, state State) {
	require.Error(t, state.ExpectError)
	assert.ErrorIs(t, state.Result.Error, state.ExpectError)
}

func ArrangeExpectedError(t *testing.T, state State) State {
	t.Helper()
	state.ExpectError = errors.New(state.Faker.RandomStringWithLength(10))
	return state
}

func AssertCommitTransaction(t *testing.T, state State) {
	t.Helper()
	err := state.Tx.Commit(state.ctx)
	require.NoError(t, err)
}

func assertRecord(t *testing.T, ctx context.Context, state State, db DB) {
	row := db.QueryRow(ctx, `SELECT * FROM regular.instance WHERE id = $1`, state.Record.ID)
	var rec Record
	require.NoError(t, row.Scan(&rec.ID, &rec.Value))
	assert.Equal(t, state.Record, rec)
}

func AssertHasRecord(sut *Instance) groat.Then[State] {
	return func(t *testing.T, state State) {
		assertRecord(t, context.Background(), state, sut)
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

func TestInstance_Begin(t *testing.T) {
	t.Run("should be able to begin basic transaction at instance", func(t *testing.T) {
		tcs := suite.Case(t)

		tcs.Given(ArrangeContext).
			When(ActBegin(tcs.SUT)).
			Then(AssertCommitTransaction)
	})

	t.Run("should be able to fail at db side", func(t *testing.T) {
		tcs := suite.Case(t)
		tcs.Given(ArrangeContext, ArrangeExpectedError).
			When(InjectPoolMock(tcs.SUT), ActBeginWithError(pgx.TxOptions{})).
			Then(AssertExpectError)
		var tx pgx.Tx
		tx, tcs.State.Result.Error = tcs.SUT.Begin(tcs.State.ctx)
		require.Nil(t, tx)

	})
}

func TestInstance_BeginTx(t *testing.T) {
	t.Run("should be able to begin basic transaction at instance", func(t *testing.T) {
		tcs := suite.Case(t)

		tcs.Given(ArrangeContext).
			When(ActBeginTransaction(tcs.SUT)).
			Then(AssertCommitTransaction)
	})

	t.Run("should be able to fail at db side", func(t *testing.T) {
		tcs := suite.Case(t)
		tcs.Given(ArrangeContext, ArrangeExpectedError).
			When(InjectPoolMock(tcs.SUT), ActBeginTxWithError(pgx.TxOptions{})).
			Then(AssertExpectError)
		var tx pgx.Tx
		tx, tcs.State.Result.Error = tcs.SUT.BeginTx(tcs.State.ctx, pgx.TxOptions{})
		require.Nil(t, tx)

	})
}

func TestInstance_Exec(t *testing.T) {
	t.Run("should be able to execute statement in transaction", func(t *testing.T) {
		tcs := suite.Case(t)

		tcs.Given(ArrangeContext, ArrangeRecord).
			When(
				ActBeginTransaction(tcs.SUT),
				ActInsertRecord(tcs.SUT),
			).
			Then(
				AssertCommitTransaction,
				AssertHasRecord(tcs.SUT),
			)
	})

	t.Run("should be able to execute statement without begin transaction", func(t *testing.T) {
		tcs := suite.Case(t)

		tcs.Given(ArrangeContext, ArrangeRecord).
			When(ActInsertRecord(tcs.SUT)).
			Then(AssertHasRecord(tcs.SUT))
	})
	t.Run("should be able fail at exec when fail transaction", func(t *testing.T) {
		tcs := suite.Case(t)
		tcs.Given(ArrangeContext, ArrangeExpectedError).
			When(
				InjectPoolMock(tcs.SUT),
				ExpectExecError("SELECT 1"),
			).Then(AssertExpectError)
		_, tcs.State.Result.Error = tcs.SUT.Exec(tcs.State.ctx, "SELECT 1")
	})
}

func TestInstance_Query(t *testing.T) {
	t.Run("should be able to find record in query", func(t *testing.T) {
		tcs := suite.Case(t)

		tcs.Given(ArrangeContext, ArrangeRecord).
			When(
				ActBeginTransaction(tcs.SUT),
				ActInsertRecord(tcs.SUT),
			).
			Then(
				AssertCommitTransaction,
				AssertRecordQueried(tcs.SUT),
			)
	})

	t.Run("should be able to find record in query in transaction", func(t *testing.T) {
		tcs := suite.Case(t)

		tcs.Given(ArrangeContext, ArrangeRecord).
			When(
				ActBeginTransaction(tcs.SUT),
				ActInsertRecord(tcs.SUT),
				ActQueryRecord(tcs.SUT),
			).
			Then(
				AssertCommitTransaction,
				AssertRecordQueried(tcs.SUT),
			)
	})

	t.Run("should be able return error when fail at driver side", func(t *testing.T) {
		tcs := suite.Case(t)

		tcs.Given(ArrangeContext, ArrangeExpectedError).
			When(
				InjectPoolMock(tcs.SUT),
				ExpectQueryError("SELECT 1"),
			).Then(AssertExpectError)

		_, tcs.State.Result.Error = tcs.SUT.Query(tcs.State.ctx, "SELECT 1")
	})
}

func TestInstance_Transactional(t *testing.T) {
	t.Run("should be able to execute statement in transactional", func(t *testing.T) {
		tcs := suite.Case(t)
		tcs.Given(ArrangeContext, ArrangeRecord).
			Then(AssertNoError)

		tcs.State.Result.Error = tcs.SUT.Transactional(tcs.State.ctx, func(ctx context.Context) error {
			return nil
		})
	})

	t.Run("should be able rollback transaction", func(t *testing.T) {
		tcs := suite.Case(t)
		tcs.Given(ArrangeContext, ArrangeExpectedError).Then(AssertExpectError)

		tcs.State.Result.Error = tcs.SUT.Transactional(tcs.State.ctx, func(ctx context.Context) error {
			return tcs.State.ExpectError
		})
	})

	t.Run("should be able commit by transaction matcher", func(t *testing.T) {
		tcs := suite.Case(t)
		tcs.Given(ArrangeContext, ArrangeExpectedError,
			ArrangeTxErrPassMatcher(func(ctx context.Context, err error) bool {
				if errors.Is(err, tcs.State.ExpectError) {
					return true
				}
				return false
			})).Then(AssertExpectError)
		tcs.State.Result.Error = tcs.SUT.Transactional(tcs.State.ctx, func(ctx context.Context) error {
			return tcs.State.ExpectError
		})
	})
	t.Run("should be able run transaction with options", func(t *testing.T) {
		tcs := suite.Case(t)
		tcs.Given(ArrangeContext, ArrangeRecord, ArrangeTxOptions).Then(AssertNoError)

		tcs.State.Result.Error = tcs.SUT.Transactional(tcs.State.ctx, func(ctx context.Context) error {
			return nil
		})
	})

	t.Run("should be able run nested transaction", func(t *testing.T) {
		tcs := suite.Case(t)

		tcs.Given(ArrangeContext, ArrangeRecord).
			When(ActBegin(tcs.SUT)).
			Then(AssertNoError, AssertCommitTransaction)

		tcs.State.Result.Error = tcs.SUT.Transactional(tcs.State.ctx, func(ctx context.Context) error {
			return nil
		})
	})

	t.Run("should be able run nested transaction at closed parent", func(t *testing.T) {
		tcs := suite.Case(t)

		tcs.Given(ArrangeContext, ArrangeRecord, ArrangeAsExpectError(pgx.ErrTxClosed)).
			When(ActBegin(tcs.SUT), ActCommit).
			Then(AssertExpectError)

		tcs.State.Result.Error = tcs.SUT.Transactional(tcs.State.ctx, func(ctx context.Context) error {
			return nil
		})
	})

	t.Run("should be able run nested transaction and pass by matcher", func(t *testing.T) {
		tcs := suite.Case(t)

		tcs.Given(ArrangeContext, ArrangeExpectedError, ArrangeTxErrPassMatcher(
			func(ctx context.Context, err error) bool {
				if errors.Is(err, tcs.State.ExpectError) {
					return true
				}
				return false
			})).
			When(ActBegin(tcs.SUT)).
			Then(AssertExpectError)

		tcs.State.Result.Error = tcs.SUT.Transactional(tcs.State.ctx, func(ctx context.Context) error {
			return tcs.State.ExpectError
		})
	})

	t.Run("should be able run nested transaction and rollback by error", func(t *testing.T) {
		tcs := suite.Case(t)

		tcs.Given(ArrangeContext, ArrangeExpectedError).
			When(ActBegin(tcs.SUT)).
			Then(AssertExpectError)

		tcs.State.Result.Error = tcs.SUT.Transactional(tcs.State.ctx, func(ctx context.Context) error {
			return tcs.State.ExpectError
		})
	})

	t.Run("should be able run nested transaction and rollback it", func(t *testing.T) {
		tcs := suite.Case(t)

		tcs.Given(ArrangeContext).
			When(ActBegin(tcs.SUT)).
			Then(AssertExpectError)

		tcs.State.Result.Error = tcs.SUT.Transactional(tcs.State.ctx, func(ctx context.Context) error {
			_, err := tcs.SUT.Exec(ctx, "ZELECT 1")
			tcs.State.ExpectError = err
			return err
		})
	})

	t.Run("should be able run nested transaction and fail at commit", func(t *testing.T) {
		tcs := suite.Case(t)
		tcs.Given(
			ArrangeContext,
			ArrangeTxMockInContext,
			ArrangeNestedTx,
			ArrangeExpectedError,
		).When(ActBeginNested, ActFailAtNestedCommit).Then(AssertExpectError)

		tcs.State.Result.Error = tcs.SUT.Transactional(tcs.State.ctx, func(ctx context.Context) error {
			return nil
		})
	})

	t.Run("should be able run nested transaction and fail at rollback", func(t *testing.T) {
		tcs := suite.Case(t)
		tcs.Given(
			ArrangeContext,
			ArrangeTxMockInContext,
			ArrangeNestedTx,
			ArrangeExpectedError,
		).When(ActBeginNested, ActFailRollbackAtNestedCommit).Then(AssertExpectError)

		tcs.State.Result.Error = tcs.SUT.Transactional(tcs.State.ctx, func(ctx context.Context) error {
			return nil
		})
	})
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

func ActCommit(t *testing.T, _ Deps, state State) State {
	require.NoError(t, state.Tx.Commit(context.Background()))
	return state
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

func AssertNoError(t *testing.T, state State) {
	require.NoError(t, state.Result.Error)
}
