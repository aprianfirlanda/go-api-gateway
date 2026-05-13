package health

import (
	"context"

	"syra-backend/internal/ports/output"
)

type MultiPinger struct {
	pingers []output.DBPinger
}

func NewMultiPinger(pingers ...output.DBPinger) *MultiPinger {
	filtered := make([]output.DBPinger, 0, len(pingers))
	for _, pinger := range pingers {
		if pinger != nil {
			filtered = append(filtered, pinger)
		}
	}
	return &MultiPinger{pingers: filtered}
}

func (m *MultiPinger) Ping(ctx context.Context) error {
	for _, pinger := range m.pingers {
		if err := pinger.Ping(ctx); err != nil {
			return err
		}
	}
	return nil
}
