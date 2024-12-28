package collector

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuilder_QueryPerSecond(t *testing.T) {
	t.Run("should be able to be able", func(t *testing.T) {
		exp := NewMockCounter(t)
		bld := New()
		tmp, ok := bld.(builder)
		require.True(t, ok)
		require.True(t, tmp.queryPerSeconds.IsEmpty())

		bld = bld.QueryPerSecond(func(labels ...string) (Counter, error) {
			return exp, nil
		})

		res, ok := bld.(builder)
		require.True(t, ok)
		assert.False(t, res.queryPerSeconds.IsEmpty())
		assert.True(t, tmp.queryPerSeconds.IsEmpty())

		col, err := res.queryPerSeconds.Value()
		require.NoError(t, err)
		assert.Equal(t, exp, col)
	})
}

func TestBuilder_Latency(t *testing.T) {
	t.Run("should be able to be able", func(t *testing.T) {
		exp := NewMockHistogram(t)
		bld := New()
		tmp, ok := bld.(builder)
		require.True(t, ok)
		require.True(t, tmp.queryLatency.IsEmpty())

		bld = bld.Latency(func(labels ...string) (Histogram, error) {
			return exp, nil
		})
		res, ok := bld.(builder)
		require.True(t, ok)
		assert.False(t, res.queryLatency.IsEmpty())
		assert.True(t, tmp.queryLatency.IsEmpty())

		col, err := res.queryLatency.Value()
		require.NoError(t, err)
		assert.Equal(t, exp, col)
	})
}

func TestBuilder_ErrorsLogInterceptor(t *testing.T) {
	t.Run("should be able to be able", func(t *testing.T) {
		exp := errors.New(uuid.NewString())

		bld := New()
		tmp, ok := bld.(builder)
		require.True(t, ok)
		require.True(t, tmp.logInterceptor.IsEmpty())
		var resErr error
		bld = bld.ErrorsLogInterceptor(func(err error) {
			resErr = err
		})

		res, ok := bld.(builder)
		require.True(t, ok)
		assert.False(t, res.logInterceptor.IsEmpty())
		assert.True(t, tmp.logInterceptor.IsEmpty())

		res.logInterceptor.Value(exp)
		assert.Equal(t, exp, resErr)
	})
}

func TestBuilder_ResultsInterceptor(t *testing.T) {
	t.Run("should be able to be able", func(t *testing.T) {
		exp := uuid.NewString()
		bld := New()
		tmp, ok := bld.(builder)
		require.True(t, ok)
		require.True(t, tmp.resultsInterceptor.IsEmpty())

		bld = bld.ResultsInterceptor(func(ctx context.Context, err error) string {
			return exp
		})

		res, ok := bld.(builder)
		require.True(t, ok)
		assert.False(t, res.resultsInterceptor.IsEmpty())
		assert.True(t, tmp.resultsInterceptor.IsEmpty())

		assert.Equal(t, exp, res.resultsInterceptor.Value(context.Background(), nil))
	})
}

func TestBuilder_Build(t *testing.T) {
	t.Run("should be able to be able", func(t *testing.T) {
		ltc := NewMockHistogram(t)
		cnt := NewMockCounter(t)

		var randomLogErr = errors.New(uuid.NewString())
		var randomResultErr = errors.New(uuid.NewString())
		var expString = uuid.NewString()
		var resErr error
		var logErr error

		col, err := New().
			Latency(func(labels ...string) (Histogram, error) {
				return ltc, nil
			}).
			QueryPerSecond(func(labels ...string) (Counter, error) {
				return cnt, nil
			}).
			ErrorsLogInterceptor(func(takenErr error) {
				logErr = takenErr
			}).
			ResultsInterceptor(func(ctx context.Context, takenErr error) string {
				resErr = takenErr
				return expString
			}).
			Build()
		require.NoError(t, err)

		col.logInterceptor(randomLogErr)
		assert.Equal(t, randomLogErr, logErr)

		resultLabel := col.interceptor(context.Background(), randomResultErr)
		assert.Equal(t, randomResultErr, resErr)
		assert.Equal(t, expString, resultLabel)
	})

	t.Run("should be able correct with default interceptors", func(t *testing.T) {
		ltc := NewMockHistogram(t)
		cnt := NewMockCounter(t)

		var randomLogErr = errors.New(uuid.NewString())
		var randomResultErr = errors.New(uuid.NewString())

		col, err := New().
			Latency(func(labels ...string) (Histogram, error) {
				return ltc, nil
			}).
			QueryPerSecond(func(labels ...string) (Counter, error) {
				return cnt, nil
			}).
			Build()
		require.NoError(t, err)

		col.logInterceptor(randomLogErr)

		resultLabel := col.interceptor(context.Background(), randomResultErr)
		assert.Equal(t, InterceptAsFailure, resultLabel)

		resultLabel = col.interceptor(context.Background(), nil)
		assert.Equal(t, InterceptAsSuccess, resultLabel)
	})

	t.Run("should be able failed", func(t *testing.T) {
		t.Run("when query per seconds collector is not present", func(t *testing.T) {
			col, err := New().Build()
			require.ErrorIs(t, err, ErrQueryPerSecondIsRequired)
			assert.Nil(t, col)
		})
		t.Run("when query latency collector is present", func(t *testing.T) {
			col, err := New().QueryPerSecond(func(labels ...string) (Counter, error) {
				return nil, nil
			}).Build()
			require.ErrorIs(t, err, ErrQueryLatencyIsRequired)
			assert.Nil(t, col)
		})
	})

}
