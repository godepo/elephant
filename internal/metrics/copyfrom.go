package metrics

import (
	"context"
	"time"

	"github.com/godepo/elephant/internal/pkg/pgcontext"
	"github.com/jackc/pgx/v5"
)

// CopyFrom measures the entire copy as one operation.
// The timeout is applied before the copy starts and covers reading rowSrc and sending all rows.
func (m DB) CopyFrom(
	ctx context.Context, tableName pgx.Identifier, columnNames []string, rowSrc pgx.CopyFromSource,
) (int64, error) {
	begin := time.Now()
	if timeout, ok := pgcontext.QueryTimeoutFrom(ctx); ok {
		cancelCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		ctx = cancelCtx
	}
	rows, err := m.db.CopyFrom(ctx, tableName, columnNames, rowSrc)
	m.defaultMetricsCollector.TrackQueryMetrics(ctx, begin, err)
	return rows, err
}
