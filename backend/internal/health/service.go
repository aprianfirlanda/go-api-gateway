package health

import (
	"context"

	"backend/internal/ports/input"
	"backend/internal/ports/output"
)

type Service struct {
	db output.DBPinger
}

func NewService(db output.DBPinger) input.HealthService {
	return &Service{db: db}
}

func (s *Service) Liveness(ctx context.Context) error {
	return ctx.Err()
}

func (s *Service) Readiness(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s.db == nil {
		return nil
	}
	return s.db.Ping(ctx)
}
