package collector

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/godepo/elephant/internal/pkg/pgcontext"
	"github.com/godepo/groat"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type Given struct {
	Labels      []string
	ctx         context.Context
	ResultLabel string
	Error       error
	TimeMark    time.Time
}

type Calls struct {
	ErrAtQueryPerSecondCollector error
	ErrAtQueryLatencyCollector   error
}
type State struct {
	Given Given
	Calls Calls
}

type Deps struct {
	MockErrorsLogsInterceptor   *MockErrorsLogInterceptor
	MockResultsInterceptor      *MockInterceptor
	MockQueryPerSecondCollector *MockCounterCollector
	MockQueryLatencyCollector   *MockHistogramCollector
	MockQueryCounter            *MockCounter
	MockLatencyHistogram        *MockHistogram
}

func newCase(t *testing.T) *groat.Case[Deps, State, *Collector] {
	tc := groat.New[Deps, State, *Collector](
		t,
		func(t *testing.T, deps Deps) *Collector {
			res, err := New().
				ErrorsLogInterceptor(deps.MockErrorsLogsInterceptor.Execute).
				ResultsInterceptor(deps.MockResultsInterceptor.Execute).
				QueryPerSecond(deps.MockQueryPerSecondCollector.Execute).
				Latency(deps.MockQueryLatencyCollector.Execute).
				Build()
			require.NoError(t, err)
			return res
		},
		func(t *testing.T, deps Deps) Deps {
			deps.MockErrorsLogsInterceptor = NewMockErrorsLogInterceptor(t)
			deps.MockResultsInterceptor = NewMockInterceptor(t)
			deps.MockQueryPerSecondCollector = NewMockCounterCollector(t)
			deps.MockQueryLatencyCollector = NewMockHistogramCollector(t)
			deps.MockQueryCounter = NewMockCounter(t)
			deps.MockLatencyHistogram = NewMockHistogram(t)
			return deps
		},
	)

	tc.Go()

	return tc
}

func TestCollector_TrackQueryMetrics(t *testing.T) {
	t.Run("should be able to be able", func(t *testing.T) {
		tc := newCase(t)
		tc.Given(
			ArrangeSecondsBeforeTimeMark,
			ArrangeQueryLabels,
			ArrangeContext,
			ArrangeLabeledContext,
			ArrangeResultLabel,
		).
			When(ActInterceptResult,
				ActQueryPerSecondCollector,
				ActQueryCounter,
				ActQueryLatencyCollector,
				ActQueryLatencyObserve,
			)
		tc.SUT.TrackQueryMetrics(tc.State.Given.ctx, tc.State.Given.TimeMark, nil)
	})

	t.Run("should be able to be able when fail get query counter", func(t *testing.T) {
		tc := newCase(t)
		tc.Given(
			ArrangeSecondsBeforeTimeMark,
			ArrangeQueryLabels,
			ArrangeContext,
			ArrangeLabeledContext,
			ArrangeResultLabel,
			ArrangeErrAtCollectQueryCounter,
		).
			When(ActInterceptResult,
				ActQueryPerSecondCollector,
				ActLogAtQueryCounter,
				ActQueryLatencyCollector,
				ActQueryLatencyObserve,
			)
		tc.SUT.TrackQueryMetrics(tc.State.Given.ctx, tc.State.Given.TimeMark, nil)
	})

	t.Run("should be able to be able when fail get query latency histogram", func(t *testing.T) {
		tc := newCase(t)
		tc.Given(
			ArrangeSecondsBeforeTimeMark,
			ArrangeQueryLabels,
			ArrangeContext,
			ArrangeLabeledContext,
			ArrangeResultLabel,
			ArrangeErrAtCollectQueryLatencyHistogram,
		).
			When(ActInterceptResult,
				ActQueryPerSecondCollector,
				ActQueryCounter,
				ActQueryLatencyCollector,
				AcLogAtQueryLatency,
			)
		tc.SUT.TrackQueryMetrics(tc.State.Given.ctx, tc.State.Given.TimeMark, nil)
	})
	t.Run("should be able to do nothing, when labels is empty", func(t *testing.T) {
		tc := newCase(t)
		tc.Given(
			ArrangeSecondsBeforeTimeMark,
			ArrangeContext,
		)
		tc.SUT.TrackQueryMetrics(tc.State.Given.ctx, tc.State.Given.TimeMark, nil)
	})
}

func AcLogAtQueryLatency(t *testing.T, deps Deps, state State) State {
	deps.MockErrorsLogsInterceptor.EXPECT().
		Execute(mock.Anything).
		Run(
			func(err error) {
				require.ErrorIs(t, err, state.Calls.ErrAtQueryLatencyCollector)
				assert.ErrorIs(t, err, ErrCantQetQueryLatencyCollector)
			})
	return state
}

func ArrangeErrAtCollectQueryLatencyHistogram(t *testing.T, state State) State {
	t.Helper()
	state.Calls.ErrAtQueryLatencyCollector = errors.New(uuid.NewString())
	return state
}

func ActLogAtQueryCounter(t *testing.T, deps Deps, state State) State {
	t.Helper()
	deps.MockErrorsLogsInterceptor.EXPECT().
		Execute(mock.Anything).Run(func(err error) {
		require.ErrorIs(t, err, state.Calls.ErrAtQueryPerSecondCollector)
		assert.ErrorIs(t, err, ErrCantGetQueryPerSecondCollector)
	})
	return state
}

func ArrangeErrAtCollectQueryCounter(t *testing.T, state State) State {
	t.Helper()
	state.Calls.ErrAtQueryPerSecondCollector = errors.New(uuid.NewString())
	return state
}

func ArrangeSecondsBeforeTimeMark(t *testing.T, state State) State {
	t.Helper()
	state.Given.TimeMark = time.Now().Truncate(time.Second)
	return state
}

func ActQueryLatencyObserve(t *testing.T, deps Deps, state State) State {
	t.Helper()
	deps.MockLatencyHistogram.EXPECT().Observe(mock.Anything).Run(func(since float64) {
		assert.GreaterOrEqual(t, float64(time.Second), since)
		assert.LessOrEqual(t, since, float64(time.Since(state.Given.TimeMark).Milliseconds()))
	})
	return state
}

func ActQueryLatencyCollector(t *testing.T, deps Deps, state State) State {
	t.Helper()
	deps.MockQueryLatencyCollector.EXPECT().Execute(labelsToAnySlice(state)...).
		Return(deps.MockLatencyHistogram, state.Calls.ErrAtQueryLatencyCollector)
	return state
}

func ActQueryCounter(t *testing.T, deps Deps, state State) State {
	t.Helper()
	deps.MockQueryCounter.EXPECT().Inc()
	return state
}

func ActQueryPerSecondCollector(t *testing.T, deps Deps, state State) State {
	t.Helper()

	result := labelsToAnySlice(state)

	deps.MockQueryPerSecondCollector.EXPECT().Execute(result...).
		Return(deps.MockQueryCounter, state.Calls.ErrAtQueryPerSecondCollector)
	return state
}

func labelsToAnySlice(state State) []any {
	result := make([]any, 0, len(state.Given.Labels)+1)
	for _, v := range append(state.Given.Labels, state.Given.ResultLabel) {
		result = append(result, v)
	}
	return result
}

func ArrangeResultLabel(t *testing.T, state State) State {
	t.Helper()
	state.Given.ResultLabel = uuid.NewString()
	return state
}

func ActInterceptResult(t *testing.T, deps Deps, state State) State {
	t.Helper()
	deps.MockResultsInterceptor.EXPECT().
		Execute(state.Given.ctx, state.Given.Error).
		Return(state.Given.ResultLabel)
	return state
}

func ArrangeContext(t *testing.T, state State) State {
	t.Helper()
	state.Given.ctx = context.Background()
	return state
}

func ArrangeLabeledContext(t *testing.T, state State) State {
	t.Helper()
	state.Given.ctx = pgcontext.With(
		state.Given.ctx,
		pgcontext.WithMetricsLabel(state.Given.Labels...),
	)
	return state
}

func ArrangeQueryLabels(t *testing.T, state State) State {
	t.Helper()
	state.Given.Labels = []string{uuid.NewString(), uuid.NewString()}
	return state
}
