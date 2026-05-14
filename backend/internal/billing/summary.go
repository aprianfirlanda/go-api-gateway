package billing

import (
	"context"
	"math"
	"time"
)

type Aggregator struct {
	store UsageEventStore
}

func NewAggregator(store UsageEventStore) *Aggregator {
	return &Aggregator{store: store}
}

func (a *Aggregator) Summarize(ctx context.Context, tenantID string, plan BillingPlan, periodStart time.Time, periodEnd time.Time) (BillingSummary, error) {
	events, err := a.store.List(ctx, UsageEventFilter{
		TenantID: tenantID,
		From:     &periodStart,
		To:       &periodEnd,
	})
	if err != nil {
		return BillingSummary{}, err
	}

	summary := BillingSummary{
		TenantID:         tenantID,
		PlanID:           plan.ID,
		BillingPeriod:    periodStart.Format("2006-01"),
		PeriodStart:      periodStart,
		PeriodEnd:        periodEnd,
		MonthlyFee:       plan.MonthlyFee,
		Currency:         plan.Currency,
		IncludedRequests: plan.IncludedRequests,
		Status:           "draft",
		CalculatedAt:     time.Now().UTC(),
	}

	for _, event := range events {
		summary.TotalRequests++
		if event.Billable {
			summary.BillableRequests++
		}
		switch event.Status {
		case StatusRejected:
			summary.RejectedRequests++
		case StatusFailed:
			summary.FailedRequests++
		case StatusTimeout:
			summary.TimeoutRequests++
		}
	}

	summary.OverageRequests = maxInt64(summary.BillableRequests-plan.IncludedRequests, 0)
	summary.OverageAmount = roundMoney(float64(summary.OverageRequests) * plan.OveragePrice)
	summary.EstimatedAmount = roundMoney(plan.MonthlyFee + summary.OverageAmount)
	return summary, nil
}

func maxInt64(a int64, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func roundMoney(value float64) float64 {
	return math.Round(value*10000) / 10000
}
