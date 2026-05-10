package output

import "context"

type DBPinger interface {
	Ping(ctx context.Context) error
}
