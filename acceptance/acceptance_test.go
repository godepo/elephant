package acceptance

import (
	"context"
	"testing"

	"github.com/godepo/elephant/internal/metrics"
	"github.com/godepo/elephant/shardedpg"
	"github.com/stretchr/testify/require"
)

func TestShardedDB(t *testing.T) {
	t.Run("should be able to be able", func(t *testing.T) {
		t.Run("when metrics collector instrumented for sharded db with single pg node", func(t *testing.T) {
			col := NewMockCollector(t)
			pool := NewMockPool(t)

			instrumentedDB := metrics.New(pool, col)

			db, err := shardedpg.New(1).
				Shard(0, instrumentedDB).
				Picker(func(ctx context.Context, key string) uint {
					return 0
				}).
				Go()
			require.NoError(t, err)
			require.NotNil(t, db)
		})
	})

}
