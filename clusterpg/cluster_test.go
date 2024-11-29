package clusterpg

import (
	"context"
	"testing"

	"github.com/godepo/groat"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

type CaseResult struct {
	Error   error
	Cluster Pool
	Rows    pgx.Rows
}

type State struct {
	LeaderPool            *MockPool
	Followers             []*MockPool
	Result                CaseResult
	Context               context.Context
	Rows                  *MockRows
	LeaderConstructor     ConstructDB
	FollowersConstructors []ConstructDB
	ExpectError           error
}

type Deps struct {
	LeaderPool         *MockPool
	FirstFollowerPool  *MockPool
	SecondFollowerPool *MockPool
}

type TestCase = *groat.Case[Deps, State, Builder]

func newTestCase(t *testing.T) TestCase {
	t.Helper()

	tc := groat.New[Deps, State, Builder](t, func(t *testing.T, deps Deps) Builder {
		b := New()
		return b
	}, func(t *testing.T, deps Deps) Deps {
		deps.LeaderPool = NewMockPool(t)
		deps.FirstFollowerPool = NewMockPool(t)
		deps.SecondFollowerPool = NewMockPool(t)
		return deps
	})
	tc.Given(func(t *testing.T, state State) State {
		state.Context = context.Background()
		return state
	})
	tc.Go()

	return tc
}

func TestNew(t *testing.T) {
	t.Run("should be able query  from cluster leader", func(t *testing.T) {
		tc := newTestCase(t)

		tc.Given(
			ArrangeWriteContext,
			ArrangeLeader(tc.Deps.LeaderPool),
			ArrangeFollower(tc.Deps.FirstFollowerPool),
			ArrangeFollower(tc.Deps.SecondFollowerPool),
			ArrangeRows,
		).When(ActWriteQuery).
			Then(AssertNoError, AssertRows)

		tc.State.Result.Cluster, tc.State.Result.Error = tc.SUT.
			Leader(tc.State.LeaderConstructor).
			Follower(tc.State.FollowersConstructors...).
			Go()
		require.NoError(t, tc.State.Result.Error)

		tc.State.Result.Rows, tc.State.Result.Error = tc.State.Result.Cluster.Query(tc.State.Context, testQuery)
	})

	t.Run("should be able query from cluster follower", func(t *testing.T) {
		tc := newTestCase(t)

		tc.Given(
			ArrangeLeader(tc.Deps.LeaderPool),
			ArrangeFollower(tc.Deps.FirstFollowerPool),
			ArrangeFollower(tc.Deps.SecondFollowerPool),
			ArrangeRows,
		).When(ActFollowerQuery(1)).
			Then(AssertNoError, AssertRows)

		tc.State.Result.Cluster, tc.State.Result.Error = tc.SUT.
			Leader(tc.State.LeaderConstructor).
			Follower(tc.State.FollowersConstructors...).
			Go()
		require.NoError(t, tc.State.Result.Error)

		tc.State.Result.Rows, tc.State.Result.Error = tc.State.Result.Cluster.Query(tc.State.Context, testQuery)
	})

	t.Run("should be able query from single cluster follower", func(t *testing.T) {
		tc := newTestCase(t)

		tc.Given(
			ArrangeLeader(tc.Deps.LeaderPool),
			ArrangeFollower(tc.Deps.FirstFollowerPool),
			ArrangeRows,
		).
			When(ActFollowerQuery(0)).
			Then(AssertNoError, AssertRows)

		tc.State.Result.Cluster, tc.State.Result.Error = tc.SUT.
			Leader(tc.State.LeaderConstructor).
			Follower(tc.State.FollowersConstructors...).
			Go()
		require.NoError(t, tc.State.Result.Error)

		tc.State.Result.Rows, tc.State.Result.Error = tc.State.Result.Cluster.Query(tc.State.Context, testQuery)
	})

	t.Run("should be able failed to constructing leader pool", func(t *testing.T) {
		tc := newTestCase(t)

		tc.Given(
			ArrangeExpectError,
			ArrangeFailedLeader,
			ArrangeFollower(tc.Deps.FirstFollowerPool),
		).Then(AssertExpectError)

		tc.State.Result.Cluster, tc.State.Result.Error = tc.SUT.
			Leader(tc.State.LeaderConstructor).
			Follower(tc.State.FollowersConstructors...).
			Go()

	})

	t.Run("should be able failed to constructing follower pool", func(t *testing.T) {
		tc := newTestCase(t)

		tc.Given(
			ArrangeExpectError,
			ArrangeLeader(tc.Deps.LeaderPool),
			ArrangeFailedFollower,
		).Then(AssertExpectError)

		tc.State.Result.Cluster, tc.State.Result.Error = tc.SUT.
			Leader(tc.State.LeaderConstructor).
			Follower(tc.State.FollowersConstructors...).
			Go()

	})

	t.Run("should be able failed  constructing empty leader constructor", func(t *testing.T) {
		tc := newTestCase(t)

		tc.Given(
			ArrangeSpecifiedError(ErrInvalidClusterConfiguration),
			ArrangeFailedFollower,
		).Then(AssertExpectError)

		tc.State.Result.Cluster, tc.State.Result.Error = tc.SUT.
			Follower(tc.State.FollowersConstructors...).
			Go()
	})

	t.Run("should be able failed constructing empty followers constructors list", func(t *testing.T) {
		tc := newTestCase(t)

		tc.Given(
			ArrangeSpecifiedError(ErrInvalidClusterConfiguration),
		).Then(AssertExpectError)

		tc.State.Result.Cluster, tc.State.Result.Error = tc.SUT.
			Go()
	})
}
