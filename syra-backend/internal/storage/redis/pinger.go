package redis

import (
	"context"

	goredis "github.com/redis/go-redis/v9"
)

type Pinger struct {
	client *goredis.Client
}

func NewPinger(client *goredis.Client) *Pinger {
	return &Pinger{client: client}
}

func (p *Pinger) Ping(ctx context.Context) error {
	if p == nil || p.client == nil {
		return nil
	}
	return p.client.Ping(ctx).Err()
}
