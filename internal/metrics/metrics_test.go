package metrics

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDB_Begin(t *testing.T) {
	t.Run("should be able to be able", func(t *testing.T) {
		tc := newCase(t).
			Given(ArrangeContext, ArrangeReturnDBBegin).
			When(ActBegin).
			Then(AssertNoError, AssertExpectedTx)

		tc.State.Result.Tx, tc.State.Result.Error = tc.SUT.
			Begin(tc.State.Given.ctx)
	})

	t.Run("should be able to be fail", func(t *testing.T) {
		tc := newCase(t).
			Given(ArrangeContext, ArrangeDBBeginFailure).
			When(ActBegin).
			Then(AssertExpectedError, AssertExpectedTx)

		tc.State.Result.Tx, tc.State.Result.Error = tc.SUT.
			Begin(tc.State.Given.ctx)
	})

}

func TestDB_BeginTx(t *testing.T) {
	t.Run("should be able to be able", func(t *testing.T) {
		tc := newCase(t).
			Given(ArrangeContext, ArrangeReturnDBBeginTx).
			When(ActBeginTx).
			Then(AssertNoError, AssertExpectedTx)

		tc.State.Result.Tx, tc.State.Result.Error = tc.SUT.
			BeginTx(tc.State.Given.ctx, tc.State.Given.TxOptions)
	})

	t.Run("should be able to be fail", func(t *testing.T) {
		tc := newCase(t).
			Given(ArrangeContext, ArrangeDBBeginTxFailure).
			When(ActBeginTx).
			Then(AssertExpectedError, AssertExpectedTx)

		tc.State.Result.Tx, tc.State.Result.Error = tc.SUT.
			BeginTx(tc.State.Given.ctx, tc.State.Given.TxOptions)
	})
}

func TestDB_Exec(t *testing.T) {
	t.Run("should be able to be able", func(t *testing.T) {
		tc := newCase(t).
			Given(ArrangeContext, ArrangeQuery, ArrangeQueryArgs, ArrangeReturnExecTag).
			When(
				ActExec,
				ActTrackQueryMetricsForExec,
			).
			Then(AssertNoError, AssertExecTag)

		tc.State.Result.ExecTag, tc.State.Result.Error = tc.SUT.
			Exec(
				tc.State.Given.ctx,
				tc.State.Given.Query,
				tc.State.Given.QueryArgs...,
			)
	})

	t.Run("should be able to be failed", func(t *testing.T) {
		tc := newCase(t).
			Given(ArrangeContext, ArrangeQuery, ArrangeQueryArgs, ArrangeExecTagFailure).
			When(
				ActExec,
				ActTrackQueryMetricsForExec,
			).
			Then(AssertExpectedError)

		tc.State.Result.ExecTag, tc.State.Result.Error = tc.SUT.
			Exec(
				tc.State.Given.ctx,
				tc.State.Given.Query,
				tc.State.Given.QueryArgs...,
			)
	})
}

func TestDB_QueryRow(t *testing.T) {
	t.Run("should be able to be able", func(t *testing.T) {
		tc := newCase(t).
			Given(
				ArrangeContext,
				ArrangeQuery,
				ArrangeQueryArgs,
				ArrangeReturnQueryRow,
				ArrangeRowScanArgs,
			).
			When(ActDBQueryRow, ActScan, ActTrackQueryMetricsForQueryRow).
			Then(AssertNoError, AssertRowScan)

		tc.State.Result.Row = tc.SUT.
			QueryRow(tc.State.Given.ctx, tc.State.Given.Query, tc.State.Given.QueryArgs...)

		AssertDBQueryRow(t, tc.State)

		tc.State.Result.Error = tc.State.Result.Row.Scan(tc.State.Given.RowScanArgs...)
	})

	t.Run("should be able to be able when timeout specified", func(t *testing.T) {
		tc := newCase(t).
			Given(
				ArrangeContext,
				ArrangeTimeout,
				ArrangeQuery,
				ArrangeQueryArgs,
				ArrangeReturnQueryRow,
				ArrangeRowScanArgs,
			).
			When(ActDBQueryRow, ActScan, ActTrackQueryMetricsForQueryRow).
			Then(AssertNoError, AssertRowScan)

		tc.State.Result.Row = tc.SUT.
			QueryRow(tc.State.Given.ctx, tc.State.Given.Query, tc.State.Given.QueryArgs...)

		AssertDBQueryRowWithTimeout(t, tc.State)

		tc.State.Result.Error = tc.State.Result.Row.Scan(tc.State.Given.RowScanArgs...)
	})
}

func TestDB_Query(t *testing.T) {
	t.Run("should be able to be able", func(t *testing.T) {
		t.Run("when context has'not timeout", func(t *testing.T) {
			tc := newCase(t).
				Given(
					ArrangeContext,
					ArrangeQuery,
					ArrangeQueryArgs,
					ArrangeReturnQuery,
					ArrangeRowScanArgs,
				).
				When(
					ActDBQuery,
					ActRowsScan,
					ActTrackQueryMetricsForQuery,
					ActRowsClose,
					ActRowsErr,
				).
				Then(AssertNoError)

			tc.State.Result.Rows, tc.State.Result.Error = tc.SUT.
				Query(tc.State.Given.ctx, tc.State.Given.Query, tc.State.Given.QueryArgs...)

			AssertDBQuery(t, tc.State)

			tc.State.Result.Error = tc.State.Result.Rows.Scan(tc.State.Given.RowScanArgs...)
			AssertRowsClose(t, tc.State)
		})

		t.Run("when context has timeout", func(t *testing.T) {
			tc := newCase(t).
				Given(
					ArrangeContext,
					ArrangeTimeout,
					ArrangeQuery,
					ArrangeQueryArgs,
					ArrangeReturnQuery,
					ArrangeRowScanArgs,
				).
				When(
					ActDBQuery,
					ActRowsScan,
					ActTrackQueryMetricsForQuery,
					ActRowsClose,
					ActRowsErr,
				).
				Then(AssertNoError)

			tc.State.Result.Rows, tc.State.Result.Error = tc.SUT.
				Query(tc.State.Given.ctx, tc.State.Given.Query, tc.State.Given.QueryArgs...)

			AssertDBQueryWithCancel(t, tc.State)

			tc.State.Result.Error = tc.State.Result.Rows.Scan(tc.State.Given.RowScanArgs...)
			AssertRowsClose(t, tc.State)
		})
	})

	t.Run("should be able to be failed", func(t *testing.T) {
		t.Run("when query db method fails", func(t *testing.T) {
			tc := newCase(t).
				Given(
					ArrangeContext,
					ArrangeQuery,
					ArrangeQueryArgs,
					ExpectError,
					ArrangeFailDBQuery,
				).
				When(
					ActDBQuery,
					ActTrackQueryMetricsForQuery,
				).
				Then(AssertExpectedError)

			tc.State.Result.Rows, tc.State.Result.Error = tc.SUT.
				Query(tc.State.Given.ctx, tc.State.Given.Query, tc.State.Given.QueryArgs...)
		})

	})
}

func TestDB_Transactional(t *testing.T) {
	t.Run("should be able to be able", func(t *testing.T) {
		tc := newCase(t).
			Given(ArrangeContext).
			When(ActTransactional)

		require.NoError(t, tc.SUT.Transactional(tc.State.Given.ctx, func(ctx context.Context) error {
			return nil
		}))
	})
	t.Run("should be able to be failed", func(t *testing.T) {
		t.Run("when callable return error", func(t *testing.T) {
			tc := newCase(t).
				Given(ArrangeContext, ExpectError).
				When(ActTransactional).
				Then(AssertExpectedError)
			tc.State.Result.Error = tc.SUT.Transactional(
				tc.State.Given.ctx,
				func(ctx context.Context) error {
					return tc.State.Expect.Error
				})
		})

		t.Run("when callable return error", func(t *testing.T) {
			tc := newCase(t).
				Given(ArrangeContext, ExpectError).
				When(ActTransactional).
				Then(AssertExpectedError)
			tc.State.Result.Error = tc.SUT.Transactional(
				tc.State.Given.ctx,
				func(ctx context.Context) error {
					return tc.State.Expect.Error
				})
		})

		t.Run("when transactional method return error", func(t *testing.T) {
			tc := newCase(t).
				Given(ArrangeContext, ExpectError, ArrangeFailTransactional).
				When(ActTransactional).
				Then(AssertExpectedError)

			tc.State.Result.Error = tc.SUT.Transactional(
				tc.State.Given.ctx,
				func(ctx context.Context) error {
					return tc.State.Expect.Error
				})
		})
	})
}
