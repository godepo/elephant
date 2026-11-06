//go:generate go tool mockery
package poolstats

import (
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	collector := New()
	collector.Collect(nil)
}

func TestCollector_Collect(t *testing.T) {
	t.Run("should be able to be able", func(t *testing.T) {
		inf := NewMockInformer(t)
		reg := prometheus.NewRegistry()

		inf.EXPECT().AcquireCount().Return(1)
		inf.EXPECT().AcquireDuration().Return(time.Millisecond)
		inf.EXPECT().AcquiredConns().Return(1)
		inf.EXPECT().CanceledAcquireCount().Return(1)
		inf.EXPECT().ConstructingConns().Return(1)
		inf.EXPECT().EmptyAcquireCount().Return(1)
		inf.EXPECT().EmptyAcquireWaitTime().Return(time.Minute)
		inf.EXPECT().IdleConns().Return(1)
		inf.EXPECT().MaxConns().Return(1)
		inf.EXPECT().TotalConns().Return(1)
		inf.EXPECT().NewConnsCount().Return(1)
		inf.EXPECT().MaxLifetimeDestroyCount().Return(1)
		inf.EXPECT().MaxIdleDestroyCount().Return(1)

		collector := New(
			WithLabels(prometheus.Labels{"db": "pg"}),
			WithLabelsNames("shard", "role"),
			WithInformerProvider(InformerProviderFrom(inf), "0", "master"),
			WithInformerProvider(InformerProviderFrom(inf), "0", "slave_1"),
			WithInformerProvider(InformerProviderFrom(inf), "0", "slave_2"),
			WithInformerProvider(InformerProviderFrom(inf), "1", "master"),
		)

		wg := &sync.WaitGroup{}
		wg.Go(
			func() {
				require.NoError(t, reg.Register(collector))
			})

		wg.Wait()

		_, err := reg.Gather()
		require.NoError(t, err)
	})

}
