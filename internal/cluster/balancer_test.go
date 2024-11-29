package cluster

import (
	"testing"

	"github.com/godepo/groat"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultLoadBalancer(t *testing.T) {
	type LBDeps struct{}

	type LBState struct {
		Fellows []Pool
		Result  Pool
	}

	type testCaseLB = *groat.Case[LBDeps, LBState, LoadBalancer]

	newTestCaseLB := func(t *testing.T) testCaseLB {
		tc := groat.New[LBDeps, LBState, LoadBalancer](
			t,
			func(t *testing.T, deps LBDeps) LoadBalancer {
				return DefaultLoadBalancer()
			},
		)
		tc.Go()
		return tc
	}

	var ArrangeFellow groat.Given[LBState] = func(t *testing.T, state LBState) LBState {
		t.Helper()
		state.Fellows = append(state.Fellows, NewMockPool(t))
		return state
	}

	AssertFellow := func(ix int) groat.Then[LBState] {
		return func(t *testing.T, state LBState) {
			t.Helper()
			require.NotNil(t, state.Result)
			assert.Equal(t, state.Fellows[ix], state.Result)
		}
	}

	t.Run("should be able to return nil at empty fellows list", func(t *testing.T) {
		tc := newTestCaseLB(t)
		require.Nil(t, tc.SUT(tc.State.Fellows))
	})

	t.Run("should be able return item from single node list", func(t *testing.T) {
		tc := newTestCaseLB(t)
		tc.Given(ArrangeFellow).
			Then(AssertFellow(0))

		tc.State.Result = tc.SUT(tc.State.Fellows)
	})

	t.Run("should be able return second item from double node list", func(t *testing.T) {
		tc := newTestCaseLB(t)
		tc.Given(ArrangeFellow, ArrangeFellow).
			Then(AssertFellow(1))
		tc.State.Result = tc.SUT(tc.State.Fellows)
	})

	t.Run("should be able return first item from double node list", func(t *testing.T) {
		tc := newTestCaseLB(t)
		tc.Given(ArrangeFellow, ArrangeFellow).
			Then(AssertFellow(0))
		tc.SUT(tc.State.Fellows)
		tc.State.Result = tc.SUT(tc.State.Fellows)
	})
}
