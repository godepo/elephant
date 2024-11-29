package regular

import (
	"context"
	"errors"
	"testing"

	"github.com/godepo/groat/integration"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jaswdr/faker/v2"
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
