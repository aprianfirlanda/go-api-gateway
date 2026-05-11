package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"syra-backend/internal/billing"
	"syra-backend/pkg/ids"
)

type UsageEventStore struct {
	db queryer
}

func NewUsageEventStore(pool *pgxpool.Pool) *UsageEventStore {
	return &UsageEventStore{db: pool}
}

func (s *UsageEventStore) Save(ctx context.Context, event billing.UsageEvent) error {
	_, err := s.db.Exec(ctx, `
		INSERT INTO usage_events (
			id, event_id, tenant_id, consumer_id, api_product_id, route_id, source_protocol, target_protocol,
			status, http_status, upstream_status, latency_ms, billable, occurred_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`, ids.New(), event.EventID, event.TenantID, nullableString(event.ConsumerID), nullableString(event.APIProductID), nullableString(event.RouteID), event.SourceProtocol, event.TargetProtocol, event.Status, event.HTTPStatus, nullableString(event.UpstreamStatus), event.LatencyMs, event.Billable, event.OccurredAt)
	return err
}

func (s *UsageEventStore) List(ctx context.Context, filter billing.UsageEventFilter) ([]billing.UsageEvent, error) {
	rows, err := s.db.Query(ctx, `
		SELECT event_id, tenant_id::text, COALESCE(consumer_id::text, ''), COALESCE(api_product_id::text, ''),
			COALESCE(route_id::text, ''), source_protocol, target_protocol, status, COALESCE(http_status, 0),
			COALESCE(upstream_status, ''), latency_ms, billable, occurred_at
		FROM usage_events
		WHERE ($1 = '' OR tenant_id = NULLIF($1, '')::uuid)
			AND ($2::timestamptz IS NULL OR occurred_at >= $2)
			AND ($3::timestamptz IS NULL OR occurred_at < $3)
		ORDER BY occurred_at, id
	`, filter.TenantID, filter.From, filter.To)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events := []billing.UsageEvent{}
	for rows.Next() {
		var event billing.UsageEvent
		if err := rows.Scan(
			&event.EventID, &event.TenantID, &event.ConsumerID, &event.APIProductID, &event.RouteID,
			&event.SourceProtocol, &event.TargetProtocol, &event.Status, &event.HTTPStatus,
			&event.UpstreamStatus, &event.LatencyMs, &event.Billable, &event.OccurredAt,
		); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, rows.Err()
}

var _ billing.UsageEventStore = (*UsageEventStore)(nil)
