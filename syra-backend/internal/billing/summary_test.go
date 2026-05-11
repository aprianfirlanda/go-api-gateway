package billing

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestBillableForStatus(t *testing.T) {
	require.True(t, BillableForStatus(StatusSuccess, false))
	require.True(t, BillableForStatus(StatusFailed, true))
	require.False(t, BillableForStatus(StatusFailed, false))
	require.True(t, BillableForStatus(StatusTimeout, true))
	require.False(t, BillableForStatus(StatusTimeout, false))
	require.False(t, BillableForStatus(StatusRejected, true))
}

func TestAggregatorCalculatesOverage(t *testing.T) {
	ctx := context.Background()
	periodStart := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	periodEnd := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	store := NewInMemoryUsageEventStore(
		UsageEvent{TenantID: "tenant_1", Status: StatusSuccess, Billable: true, OccurredAt: periodStart.Add(time.Hour)},
		UsageEvent{TenantID: "tenant_1", Status: StatusSuccess, Billable: true, OccurredAt: periodStart.Add(2 * time.Hour)},
		UsageEvent{TenantID: "tenant_1", Status: StatusFailed, Billable: true, OccurredAt: periodStart.Add(3 * time.Hour)},
		UsageEvent{TenantID: "tenant_1", Status: StatusRejected, Billable: false, OccurredAt: periodStart.Add(4 * time.Hour)},
		UsageEvent{TenantID: "tenant_1", Status: StatusTimeout, Billable: true, OccurredAt: periodStart.Add(5 * time.Hour)},
		UsageEvent{TenantID: "tenant_2", Status: StatusSuccess, Billable: true, OccurredAt: periodStart.Add(time.Hour)},
		UsageEvent{TenantID: "tenant_1", Status: StatusSuccess, Billable: true, OccurredAt: periodEnd},
	)

	summary, err := NewAggregator(store).Summarize(ctx, "tenant_1", BillingPlan{
		ID:               "plan_1",
		MonthlyFee:       100,
		IncludedRequests: 2,
		OveragePrice:     0.25,
		Currency:         "USD",
	}, periodStart, periodEnd)

	require.NoError(t, err)
	require.Equal(t, int64(5), summary.TotalRequests)
	require.Equal(t, int64(4), summary.BillableRequests)
	require.Equal(t, int64(1), summary.RejectedRequests)
	require.Equal(t, int64(1), summary.FailedRequests)
	require.Equal(t, int64(1), summary.TimeoutRequests)
	require.Equal(t, int64(2), summary.OverageRequests)
	require.Equal(t, 0.5, summary.OverageAmount)
	require.Equal(t, 100.5, summary.EstimatedAmount)
	require.Equal(t, "USD", summary.Currency)
}
