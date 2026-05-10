package protocol

import "context"

type ProtocolAdapter interface {
	Name() string
	Decode(ctx context.Context, req InboundRequest) (CanonicalMessage, error)
	Encode(ctx context.Context, msg CanonicalMessage) (OutboundResponse, error)
}

type UpstreamAdapter interface {
	Name() string
	Call(ctx context.Context, target UpstreamTarget, msg CanonicalMessage) (CanonicalMessage, error)
}
