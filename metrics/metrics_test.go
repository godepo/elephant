package metrics

import (
	"context"
	"errors"
	"testing"

	"github.com/godepo/elephant/internal/pkg/pgcontext"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestCollector(t *testing.T) {
	col := Collector()
	require.NotNil(t, col)
}

func TestNew(t *testing.T) {
	p := NewMockPool(t)
	col := NewMockCollector(t)
	db := New(p, col)

	expErr := errors.New(uuid.NewString())

	require.NotNil(t, db)
	label := uuid.NewString()
	ctx := pgcontext.With(context.Background(), pgcontext.WithMetricsLabel(label))

	p.EXPECT().Query(mock.Anything, "SELECT 1").Return(nil, expErr)
	col.EXPECT().TrackQueryMetrics(ctx, mock.Anything, expErr)

	_, err := db.Query(ctx, "SELECT 1")
	require.Error(t, err)
	require.ErrorIs(t, err, expErr)
}
