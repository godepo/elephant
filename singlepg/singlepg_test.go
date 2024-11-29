package singlepg

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	t.Run("should be able to be able", func(t *testing.T) {
		pool := NewMockPool(t)
		tx := NewMockTx(t)
		pool.EXPECT().BeginTx(mock.Anything, mock.Anything).Return(tx, nil)
		tx.EXPECT().Commit(mock.Anything).Return(nil)
		tx.EXPECT().Rollback(mock.Anything).Return(nil)

		db := New(pool)
		require.NotNil(t, db)

		err := db.Transactional(context.Background(), func(ctx context.Context) error {
			return nil
		})
		require.NoError(t, err)
	})
}
