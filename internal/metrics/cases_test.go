package metrics

import (
	"context"
	"errors"
	"math/rand"
	"testing"
	"time"

	"github.com/godepo/elephant/internal/pkg/pgcontext"
	"github.com/godepo/groat"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type Deps struct {
	Pool      *MockPool
	Collector *MockCollector
}

type Calls struct {
	ResultBegin      *MockTx
	ErrBegin         error
	ResultBeginTx    *MockTx
	ErrBeginTx       error
	ExecTag          pgconn.CommandTag
	ErrExec          error
	ResultQueryRow   *MockRow
	ErrRowScan       error
	ResultQueryRows  *MockRows
	ErrQuery         error
	ErrRowsErr       error
	ErrTransactional error
}

type Given struct {
	ctx         context.Context
	TxOptions   pgx.TxOptions
	Query       string
	QueryArgs   []any
	RowScanArgs []any
	ErrRowScan  error
}

type Result struct {
	Error   error
	Tx      pgx.Tx
	ExecTag pgconn.CommandTag
	Row     pgx.Row
	Rows    pgx.Rows
}

type Expect struct {
	Tx    *MockTx
	Error error
}

type State struct {
	Given  Given
	Expect Expect
	Result Result
	Calls  Calls
}

func newCase(t *testing.T) *groat.Case[Deps, State, *DB] {
	tc := groat.New[Deps, State, *DB](t, func(t *testing.T, deps Deps) *DB {
		return New(deps.Pool, deps.Collector)
	}, func(t *testing.T, deps Deps) Deps {
		deps.Pool = NewMockPool(t)
		deps.Collector = NewMockCollector(t)
		return deps
	})
	tc.Go()

	return tc
}

func ArrangeFailTransactional(t *testing.T, state State) State {
	t.Helper()
	state.Calls.ErrTransactional = state.Expect.Error
	return state
}

func ExpectError(t *testing.T, state State) State {
	t.Helper()
	state.Expect.Error = errors.New(uuid.NewString())
	return state
}

func ArrangeFailDBQuery(t *testing.T, state State) State {
	t.Helper()
	state.Calls.ErrQuery = state.Expect.Error
	return state
}

func ArrangeReturnQuery(t *testing.T, state State) State {
	t.Helper()
	state.Calls.ResultQueryRows = NewMockRows(t)
	return state
}

func ArrangeRowScanArgs(t *testing.T, state State) State {
	t.Helper()
	state.Given.RowScanArgs = []any{uuid.NewString()}
	return state
}

func ArrangeReturnQueryRow(t *testing.T, state State) State {
	t.Helper()
	state.Calls.ResultQueryRow = NewMockRow(t)
	return state
}

func ArrangeExecTagFailure(t *testing.T, state State) State {
	t.Helper()
	state.Expect.Error = errors.New(uuid.New().String())
	state.Calls.ErrExec = state.Expect.Error
	return state
}

func ArrangeTimeout(t *testing.T, state State) State {
	t.Helper()
	state.Given.ctx = pgcontext.With(state.Given.ctx, pgcontext.WithTimeout(time.Hour))
	return state
}

func ArrangeReturnExecTag(t *testing.T, state State) State {
	t.Helper()
	state.Calls.ExecTag = pgconn.NewCommandTag(uuid.NewString())
	return state
}

func ArrangeQueryArgs(t *testing.T, state State) State {
	t.Helper()
	args := make([]any, rand.Intn(10))
	for i := range args {
		args[i] = uuid.NewString()
	}
	state.Given.QueryArgs = args
	return state
}

func ArrangeQuery(t *testing.T, state State) State {
	t.Helper()
	state.Given.Query = uuid.NewString()
	return state
}

func ArrangeDBBeginTxFailure(t *testing.T, state State) State {
	t.Helper()
	state.Calls.ErrBeginTx = errors.New(uuid.NewString())
	state.Expect.Error = state.Calls.ErrBeginTx
	return state
}

func ArrangeReturnDBBeginTx(t *testing.T, state State) State {
	t.Helper()
	state.Expect.Tx = NewMockTx(t)
	state.Calls.ResultBeginTx = state.Expect.Tx
	return state
}

func ArrangeDBBeginFailure(t *testing.T, state State) State {
	t.Helper()
	state.Calls.ErrBegin = errors.New(uuid.NewString())
	state.Expect.Error = state.Calls.ErrBegin
	return state
}

func ArrangeReturnDBBegin(t *testing.T, state State) State {
	t.Helper()
	state.Expect.Tx = NewMockTx(t)
	state.Calls.ResultBegin = state.Expect.Tx
	return state
}

func ArrangeContext(t *testing.T, state State) State {
	t.Helper()
	state.Given.ctx = context.Background()
	return state
}

func ActBegin(t *testing.T, deps Deps, state State) State {
	t.Helper()
	deps.Pool.EXPECT().Begin(state.Given.ctx).Return(state.Calls.ResultBegin, state.Calls.ErrBegin)
	return state
}

func ActBeginTx(t *testing.T, deps Deps, state State) State {
	t.Helper()
	deps.Pool.EXPECT().
		BeginTx(state.Given.ctx, state.Given.TxOptions).
		Return(state.Calls.ResultBeginTx, state.Calls.ErrBeginTx)
	return state
}

func ActDBQueryRow(t *testing.T, deps Deps, state State) State {
	t.Helper()
	if len(state.Given.QueryArgs) > 0 {
		deps.Pool.EXPECT().
			QueryRow(state.Given.ctx, state.Given.Query, state.Given.QueryArgs).
			Return(state.Calls.ResultQueryRow)
	} else {
		deps.Pool.EXPECT().
			QueryRow(state.Given.ctx, state.Given.Query).
			Return(state.Calls.ResultQueryRow)
	}
	return state
}

func ActTrackQueryMetricsForExec(t *testing.T, deps Deps, state State) State {
	t.Helper()
	deps.Collector.EXPECT().
		TrackQueryMetrics(state.Given.ctx, mock.Anything, state.Calls.ErrExec)
	return state
}

func ActScan(t *testing.T, _ Deps, state State) State {
	t.Helper()
	state.Calls.ResultQueryRow.EXPECT().
		Scan(state.Given.RowScanArgs).
		Return(state.Given.ErrRowScan)
	return state
}

func ActTrackQueryMetricsForQueryRow(t *testing.T, deps Deps, state State) State {
	t.Helper()
	deps.Collector.EXPECT().TrackQueryMetrics(mock.Anything, mock.Anything, state.Calls.ErrRowScan)
	return state
}

func ActRowsClose(t *testing.T, _ Deps, state State) State {
	t.Helper()
	state.Calls.ResultQueryRows.EXPECT().Close()
	return state
}

func ActRowsScan(t *testing.T, _ Deps, state State) State {
	t.Helper()
	state.Calls.ResultQueryRows.EXPECT().
		Scan(state.Given.RowScanArgs).
		Return(state.Calls.ErrRowScan)
	return state
}

func ActTrackQueryMetricsForQuery(t *testing.T, deps Deps, state State) State {
	t.Helper()
	deps.Collector.EXPECT().
		TrackQueryMetrics(mock.Anything, mock.Anything, state.Expect.Error)
	return state
}

func ActDBQuery(t *testing.T, deps Deps, state State) State {
	t.Helper()
	if len(state.Given.QueryArgs) > 0 {
		deps.Pool.EXPECT().
			Query(state.Given.ctx, state.Given.Query, state.Given.QueryArgs).
			Return(state.Calls.ResultQueryRows, state.Calls.ErrQuery)
	} else {
		deps.Pool.EXPECT().
			Query(state.Given.ctx, state.Given.Query).
			Return(state.Calls.ResultQueryRows, state.Calls.ErrQuery)
	}

	return state
}

func ActTransactional(t *testing.T, deps Deps, state State) State {
	t.Helper()
	if state.Calls.ErrTransactional != nil {
		deps.Pool.EXPECT().Transactional(state.Given.ctx, mock.Anything).
			Return(state.Calls.ErrTransactional)
		return state
	}
	deps.Pool.EXPECT().Transactional(state.Given.ctx, mock.Anything).RunAndReturn(
		func(ctx context.Context, f func(context.Context) error) error {
			return f(ctx)
		})
	return state
}

func ActRowsErr(t *testing.T, _ Deps, state State) State {
	t.Helper()
	state.Calls.ResultQueryRows.EXPECT().Err().Return(state.Calls.ErrRowsErr)
	return state
}

func ActExec(t *testing.T, deps Deps, state State) State {
	t.Helper()
	if len(state.Given.QueryArgs) > 0 {
		deps.Pool.EXPECT().
			Exec(state.Given.ctx, state.Given.Query, state.Given.QueryArgs).
			Return(state.Calls.ExecTag, state.Calls.ErrExec)
	} else {
		deps.Pool.EXPECT().
			Exec(state.Given.ctx, state.Given.Query).
			Return(state.Calls.ExecTag, state.Calls.ErrExec)
	}

	return state
}

func AssertRowsClose(t *testing.T, state State) {
	assert.NotPanics(t, state.Result.Rows.Close)
}

func AssertDBQuery(t *testing.T, state State) {
	t.Helper()
	require.NotNil(t, state.Result.Rows)
	dec, ok := state.Result.Rows.(decoratedMetricRows)
	require.True(t, ok)
	assert.Equal(t, state.Calls.ResultQueryRows, dec.Rows)
	assert.True(t, dec.cancel.IsEmpty())
}

func AssertDBQueryWithCancel(t *testing.T, state State) {
	t.Helper()
	require.NotNil(t, state.Result.Rows)
	dec, ok := state.Result.Rows.(decoratedMetricRows)
	require.True(t, ok)
	assert.Equal(t, state.Calls.ResultQueryRows, dec.Rows)
	assert.False(t, dec.cancel.IsEmpty())
}

func AssertRowScan(t *testing.T, state State) {
	require.NoError(t, state.Calls.ResultQueryRow.Scan(state.Given.RowScanArgs...))
}

func AssertDBQueryRowWithTimeout(t *testing.T, state State) {
	t.Helper()
	require.NotNil(t, state.Result.Row)
	dec, ok := state.Result.Row.(decoratedMetricRow)
	require.True(t, ok)
	assert.Equal(t, state.Calls.ResultQueryRow, dec.row)
	assert.False(t, dec.cancel.IsEmpty())
}

func AssertDBQueryRow(t *testing.T, state State) {
	t.Helper()
	require.NotNil(t, state.Result.Row)
	dec, ok := state.Result.Row.(decoratedMetricRow)
	require.True(t, ok)
	assert.Equal(t, state.Calls.ResultQueryRow, dec.row)
	assert.True(t, dec.cancel.IsEmpty())
}

func AssertExecTag(t *testing.T, state State) {
	assert.Equal(t, state.Calls.ExecTag, state.Result.ExecTag)
}

func AssertExpectedError(t *testing.T, state State) {
	require.ErrorIs(t, state.Result.Error, state.Expect.Error)
}

func AssertExpectedTx(t *testing.T, state State) {
	t.Helper()
	assert.Equal(t, state.Expect.Tx, state.Result.Tx)
}

func AssertNoError(t *testing.T, state State) {
	t.Helper()
	require.NoError(t, state.Result.Error)
}
