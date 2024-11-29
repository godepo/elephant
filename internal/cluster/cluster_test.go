package cluster

import (
	"context"
	"testing"

	"github.com/godepo/groat"
	"github.com/jackc/pgx/v5"
	"github.com/jaswdr/faker/v2"
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
