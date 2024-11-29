package clusterpg

import (
	"context"
	"errors"
	"testing"

	"github.com/godepo/elephant/internal/pkg/pgcontext"
	"github.com/godepo/groat"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testQuery = "SELECT 1"

func ArrangeSpecifiedError(err error) groat.Given[State] {
	return func(t *testing.T, state State) State {
		t.Helper()
		state.ExpectError = err
		return state
	}
}

func ArrangeExpectError(_ *testing.T, state State) State {
	state.ExpectError = errors.New(uuid.NewString())
	return state
}

func ArrangeFailedFollower(_ *testing.T, state State) State {
	state.FollowersConstructors = append(state.FollowersConstructors, func() (Pool, error) {
		return nil, state.ExpectError
	})
	return state
}

func ArrangeFailedLeader(t *testing.T, state State) State {
	t.Helper()
	state.LeaderConstructor = func() (Pool, error) {
		return nil, state.ExpectError
	}
	return state
}

func ArrangeRows(t *testing.T, state State) State {
	t.Helper()
	state.Rows = NewMockRows(t)
	return state
}

func ArrangeWriteContext(_ *testing.T, state State) State {
	state.Context = pgcontext.With(context.Background(), pgcontext.WithCanWrite)
	return state
}

func ArrangeFollower(pool *MockPool) groat.Given[State] {
	return func(t *testing.T, state State) State {
		state.Followers = append(state.Followers, pool)
		state.FollowersConstructors = append(state.FollowersConstructors, func() (Pool, error) {
			return pool, nil
		})
		return state
	}
}

func ArrangeLeader(pool *MockPool) groat.Given[State] {
	return func(t *testing.T, state State) State {
		state.LeaderPool = pool
		state.LeaderConstructor = func() (Pool, error) {
			return state.LeaderPool, nil
		}
		return state
	}
}

func ActFollowerQuery(selectedFollower int) groat.When[Deps, State] {
	return func(t *testing.T, deps Deps, state State) State {
		t.Helper()
		state.Followers[selectedFollower].EXPECT().Query(state.Context, testQuery).Return(state.Rows, nil)
		return state
	}

}

func ActWriteQuery(_ *testing.T, _ Deps, state State) State {
	state.LeaderPool.EXPECT().Query(state.Context, testQuery).Return(state.Rows, nil)

	return state
}

func AssertExpectError(t *testing.T, state State) {
	t.Helper()
	require.Error(t, state.Result.Error)
	assert.ErrorIs(t, state.Result.Error, state.ExpectError)
}

func AssertRows(t *testing.T, state State) {
	t.Helper()
	assert.Equal(t, state.Rows, state.Result.Rows)
}

func AssertNoError(t *testing.T, state State) {
	require.ErrorIs(t, state.Result.Error, state.ExpectError)
}
