package iso8583

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"backend/internal/protocol"
)

const Name = "iso8583"

type Adapter struct {
	codec    Codec
	profiles ProfileStore
	dialer   Dialer
}

type Dialer interface {
	DialContext(ctx context.Context, network string, address string) (net.Conn, error)
}

func NewAdapter(codec Codec, profiles ProfileStore, dialer Dialer) *Adapter {
	if codec == nil {
		codec = NewInternalCodec()
	}
	if dialer == nil {
		dialer = &net.Dialer{}
	}
	return &Adapter{codec: codec, profiles: profiles, dialer: dialer}
}

func (a *Adapter) Name() string {
	return Name
}

func (a *Adapter) Call(ctx context.Context, target protocol.UpstreamTarget, msg protocol.CanonicalMessage) (protocol.CanonicalMessage, error) {
	if strings.ToLower(target.Protocol) != Name {
		return protocol.CanonicalMessage{}, fmt.Errorf("unsupported upstream protocol %q", target.Protocol)
	}

	profileID := target.Metadata["iso8583ProfileId"]
	profile, err := a.profiles.Find(ctx, profileID)
	if err != nil {
		return protocol.CanonicalMessage{}, err
	}

	requestBytes, err := a.codec.Pack(profile, msg.Fields)
	if err != nil {
		return protocol.CanonicalMessage{}, err
	}

	conn, err := a.dialer.DialContext(ctx, "tcp", stripTCPPrefix(target.BaseURL))
	if err != nil {
		return protocol.CanonicalMessage{}, err
	}
	defer conn.Close()

	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	} else {
		_ = conn.SetDeadline(time.Now().Add(30 * time.Second))
	}

	if _, err := conn.Write(requestBytes); err != nil {
		return protocol.CanonicalMessage{}, err
	}

	buffer := make([]byte, 4096)
	n, err := conn.Read(buffer)
	if err != nil {
		return protocol.CanonicalMessage{}, err
	}

	responseFields, err := a.codec.Unpack(profile, buffer[:n])
	if err != nil {
		return protocol.CanonicalMessage{}, err
	}

	return protocol.CanonicalMessage{
		TenantID:       msg.TenantID,
		ConsumerID:     msg.ConsumerID,
		CredentialID:   msg.CredentialID,
		APIProductID:   msg.APIProductID,
		RouteID:        msg.RouteID,
		SourceProtocol: msg.SourceProtocol,
		TargetProtocol: msg.TargetProtocol,
		Operation:      msg.Operation,
		Headers:        msg.Headers,
		Fields:         responseFields,
		Metadata:       msg.Metadata,
		SensitiveKeys:  profile.SensitiveKeys,
	}, nil
}

func stripTCPPrefix(address string) string {
	return strings.TrimPrefix(address, "tcp://")
}
