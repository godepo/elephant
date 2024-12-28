package metrics

import (
	"context"
	"time"

	"github.com/godepo/elephant/internal/pkg/monads"
	"github.com/jackc/pgx/v5"
)

type decoratedMetricRow struct {
	ctx       context.Context
	begin     time.Time
	row       pgx.Row
	collector Collector
	cancel    monads.Optional[context.CancelFunc]
}

func (row decoratedMetricRow) Scan(dest ...any) error {
	defer func() {
		if !row.cancel.IsEmpty() {
			row.cancel.Value()
		}
	}()

	err := row.row.Scan(dest...)
	row.collector.TrackQueryMetrics(row.ctx, row.begin, err)
	return err
}
