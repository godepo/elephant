package trunk

import (
	"errors"
	"testing"

	"github.com/godepo/elephant"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestNode_New(t *testing.T) {
	tc := suite.Case(t)

	exp := errors.New(uuid.NewString())

	metricsBuilder := NewMockMetricsBuilder(t)
	metricsBuilder.EXPECT().Latency(mock.Anything).Return(metricsBuilder)
	metricsBuilder.EXPECT().QueryPerSecond(mock.Anything).Return(metricsBuilder)
	metricsBuilder.EXPECT().Build().Return(nil, exp)

	_, err := newNode(t.Context(), tc.Deps.Config.ConnString(),
		&Settings{
			registry: prometheus.NewRegistry(),
		},
		func(rules *roleRules) {
			rules.metricsCollectorProvider = func() elephant.MetricsBuilder {
				return metricsBuilder
			}
		})
	require.ErrorIs(t, err, exp)
}
