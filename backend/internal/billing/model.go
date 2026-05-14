package billing

import "time"

const (
	StatusSuccess  = "success"
	StatusFailed   = "failed"
	StatusRejected = "rejected"
	StatusTimeout  = "timeout"

	PlanStatusActive   = "active"
	PlanStatusInactive = "inactive"
)

type UsageEvent struct {
	EventID        string
	TenantID       string
	ConsumerID     string
	APIProductID   string
	RouteID        string
	SourceProtocol string
	TargetProtocol string
	Status         string
	HTTPStatus     int
	UpstreamStatus string
	LatencyMs      int64
	Billable       bool
	OccurredAt     time.Time
}

type UsageEventPage struct {
	Data       []UsageEvent
	NextCursor *string
}

type BillingPlan struct {
	ID               string
	Name             string
	Slug             string
	MonthlyFee       float64
	IncludedRequests int64
	OveragePrice     float64
	Currency         string
	Status           string
}

type BillingSummary struct {
	TenantID         string
	BillingPeriod    string
	PlanID           string
	PeriodStart      time.Time
	PeriodEnd        time.Time
	TotalRequests    int64
	BillableRequests int64
	IncludedRequests int64
	RejectedRequests int64
	FailedRequests   int64
	TimeoutRequests  int64
	OverageRequests  int64
	MonthlyFee       float64
	OverageAmount    float64
	EstimatedAmount  float64
	Currency         string
	Status           string
	CalculatedAt     time.Time
}

func BillableForStatus(status string, upstreamCalled bool) bool {
	switch status {
	case StatusSuccess:
		return true
	case StatusFailed, StatusTimeout:
		return upstreamCalled
	default:
		return false
	}
}
