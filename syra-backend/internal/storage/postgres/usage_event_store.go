package postgres

import (
	"context"
	"encoding/base64"
	"fmt"
	"strconv"

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
	page, err := s.ListPage(ctx, filter, 1000000, "")
	if err != nil {
		return nil, err
	}
	return page.Data, nil
}

func (s *UsageEventStore) ListPage(ctx context.Context, filter billing.UsageEventFilter, limit int, cursor string) (billing.UsageEventPage, error) {
	if limit <= 0 {
		limit = 50
	}
	offset, err := decodeCursor(cursor)
	if err != nil {
		return billing.UsageEventPage{}, err
	}
	rows, err := s.db.Query(ctx, `
		SELECT event_id, tenant_id::text, COALESCE(consumer_id::text, ''), COALESCE(api_product_id::text, ''),
			COALESCE(route_id::text, ''), source_protocol, target_protocol, status, COALESCE(http_status, 0),
			COALESCE(upstream_status, ''), latency_ms, billable, occurred_at
		FROM usage_events
		WHERE ($1 = '' OR tenant_id = NULLIF($1, '')::uuid)
			AND ($2 = '' OR route_id = NULLIF($2, '')::uuid)
			AND ($3 = '' OR consumer_id = NULLIF($3, '')::uuid)
			AND ($4 = '' OR status = $4)
			AND ($5 = '' OR source_protocol = $5)
			AND ($6 = '' OR target_protocol = $6)
			AND ($7::bool IS NULL OR billable = $7)
			AND ($8::timestamptz IS NULL OR occurred_at >= $8)
			AND ($9::timestamptz IS NULL OR occurred_at < $9)
		ORDER BY occurred_at, id
		LIMIT $10 OFFSET $11
	`, filter.TenantID, filter.RouteID, filter.ConsumerID, filter.Status, filter.SourceProtocol, filter.TargetProtocol, filter.Billable, filter.From, filter.To, limit+1, offset)
	if err != nil {
		return billing.UsageEventPage{}, err
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
			return billing.UsageEventPage{}, err
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return billing.UsageEventPage{}, err
	}
	page := billing.UsageEventPage{Data: events}
	if len(page.Data) > limit {
		page.Data = page.Data[:limit]
		next := encodeCursor(offset + limit)
		page.NextCursor = &next
	}
	return page, nil
}

var _ billing.UsageEventStore = (*UsageEventStore)(nil)

func decodeCursor(raw string) (int, error) {
	if raw == "" {
		return 0, nil
	}
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid cursor")
	}
	offset, err := strconv.Atoi(string(decoded))
	if err != nil {
		return 0, fmt.Errorf("invalid cursor")
	}
	return offset, nil
}

func encodeCursor(offset int) string {
	return base64.RawURLEncoding.EncodeToString([]byte(strconv.Itoa(offset)))
}
