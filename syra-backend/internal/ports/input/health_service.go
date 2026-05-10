package input

import "context"

type HealthService interface {
	Liveness(ctx context.Context) error
	Readiness(ctx context.Context) error
}
