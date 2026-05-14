package iso8583listener

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"backend/internal/gateway/policy"
	"backend/internal/gateway/upstream"
	"backend/internal/protocol"
	iso8583protocol "backend/internal/protocol/iso8583"
	restprotocol "backend/internal/protocol/rest"
	"backend/internal/transform"
)

type Profile struct {
	ID               string
	TenantID         string
	ConsumerID       string
	APIProductID     string
	RouteID          string
	ISO8583ProfileID string
	TemplateID       string
	RESTUpstream     upstream.Upstream
	RESTMethod       string
	RESTPath         string
	Timeout          time.Duration
}

type Server struct {
	listenerProfile Profile
	codec           iso8583protocol.Codec
	profiles        iso8583protocol.ProfileStore
	templates       transform.Store
	transforms      *transform.Engine
	rest            *restprotocol.Adapter
	policies        *policy.Pipeline
}

func NewServer(
	listenerProfile Profile,
	codec iso8583protocol.Codec,
	profiles iso8583protocol.ProfileStore,
	templates transform.Store,
	transforms *transform.Engine,
	rest *restprotocol.Adapter,
	policies *policy.Pipeline,
) *Server {
	if codec == nil {
		codec = iso8583protocol.NewInternalCodec()
	}
	if transforms == nil {
		transforms = transform.NewEngine()
	}
	if rest == nil {
		rest = restprotocol.NewAdapter(nil)
	}
	return &Server{
		listenerProfile: listenerProfile,
		codec:           codec,
		profiles:        profiles,
		templates:       templates,
		transforms:      transforms,
		rest:            rest,
		policies:        policies,
	}
}

func (s *Server) Serve(ctx context.Context, listener net.Listener) error {
	for {
		conn, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}

		go s.handleConn(ctx, conn)
	}
}

func (s *Server) handleConn(parent context.Context, conn net.Conn) {
	defer conn.Close()

	ctx := parent
	cancel := func() {}
	if s.listenerProfile.Timeout > 0 {
		ctx, cancel = context.WithTimeout(parent, s.listenerProfile.Timeout)
	}
	defer cancel()

	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}

	response, err := s.HandleMessage(ctx, conn, conn.RemoteAddr().String())
	if err != nil {
		return
	}
	_, _ = conn.Write(response)
}

func (s *Server) HandleMessage(ctx context.Context, reader io.Reader, remoteAddr ...string) ([]byte, error) {
	profile, err := s.profiles.Find(ctx, s.listenerProfile.ISO8583ProfileID)
	if err != nil {
		return nil, err
	}

	raw, err := readISOMessage(reader, profile)
	if err != nil {
		return nil, err
	}

	if err := s.policies.Evaluate(ctx, policy.Request{
		TenantID:   s.listenerProfile.TenantID,
		ConsumerID: s.listenerProfile.ConsumerID,
		RouteID:    s.listenerProfile.RouteID,
		Protocol:   iso8583protocol.Name,
		RemoteAddr: firstString(remoteAddr),
		SizeBytes:  int64(len(raw)),
	}); err != nil {
		return nil, err
	}

	fields, err := s.codec.Unpack(profile, raw)
	if err != nil {
		return nil, err
	}

	template, err := s.templates.Find(ctx, s.listenerProfile.TenantID, s.listenerProfile.TemplateID)
	if err != nil {
		return nil, err
	}

	inbound := protocol.CanonicalMessage{
		TenantID:       s.listenerProfile.TenantID,
		ConsumerID:     s.listenerProfile.ConsumerID,
		APIProductID:   s.listenerProfile.APIProductID,
		RouteID:        s.listenerProfile.RouteID,
		SourceProtocol: iso8583protocol.Name,
		TargetProtocol: restprotocol.Name,
		Headers:        http.Header{},
		Fields:         fields,
		Metadata:       map[string]any{"listenerProfileId": s.listenerProfile.ID},
		SensitiveKeys:  profile.SensitiveKeys,
	}

	restRequest, err := s.transforms.DryRun(ctx, template, transform.DirectionRequest, inbound)
	if err != nil {
		return nil, err
	}
	restRequest.Method = defaultString(s.listenerProfile.RESTMethod, http.MethodPost)
	restRequest.Path = s.listenerProfile.RESTPath

	restResponse, err := s.rest.Call(ctx, protocol.UpstreamTarget{
		ID:       s.listenerProfile.RESTUpstream.ID,
		TenantID: s.listenerProfile.RESTUpstream.TenantID,
		Protocol: string(s.listenerProfile.RESTUpstream.Protocol),
		BaseURL:  s.listenerProfile.RESTUpstream.BaseURL,
	}, restRequest)
	if err != nil {
		return nil, err
	}

	isoResponse, err := s.transforms.DryRun(ctx, template, transform.DirectionResponse, restResponse)
	if err != nil {
		return nil, err
	}

	return s.codec.Pack(profile, isoResponse.Fields)
}

func readISOMessage(reader io.Reader, profile iso8583protocol.Profile) ([]byte, error) {
	if profile.LengthHeader {
		header := make([]byte, 2)
		if _, err := io.ReadFull(reader, header); err != nil {
			return nil, fmt.Errorf("read length header: %w", err)
		}
		length := int(binary.BigEndian.Uint16(header))
		if length <= 0 {
			return nil, errors.New("invalid length header")
		}
		body := make([]byte, length)
		if _, err := io.ReadFull(reader, body); err != nil {
			return nil, fmt.Errorf("read iso8583 body: %w", err)
		}
		return append(header, body...), nil
	}

	body, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	if len(body) == 0 {
		return nil, errors.New("empty iso8583 message")
	}
	return body, nil
}

func defaultString(value string, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func firstString(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
}
