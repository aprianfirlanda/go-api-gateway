package runtimeconfig

import (
	"context"
	"fmt"
)

type Pinger struct {
	manager *Manager
}

func NewPinger(manager *Manager) *Pinger {
	return &Pinger{manager: manager}
}

func (p *Pinger) Ping(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if p == nil || p.manager == nil {
		return fmt.Errorf("runtime config manager unavailable")
	}
	current := p.manager.Current()
	if current.Version <= 0 {
		if err := p.manager.LastError(); err != nil {
			return fmt.Errorf("runtime config load failed: %w", err)
		}
		return fmt.Errorf("runtime config has not been loaded")
	}
	if err := p.manager.LastError(); err != nil {
		return fmt.Errorf("runtime config last reload failed: %w", err)
	}
	return nil
}
