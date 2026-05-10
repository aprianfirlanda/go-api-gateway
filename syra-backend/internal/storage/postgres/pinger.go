package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Pinger struct {
	pool *pgxpool.Pool
}

func NewPinger(pool *pgxpool.Pool) *Pinger {
	return &Pinger{pool: pool}
}

func (p *Pinger) Ping(ctx context.Context) error {
	if p == nil || p.pool == nil {
		return nil
	}
	return p.pool.Ping(ctx)
}
