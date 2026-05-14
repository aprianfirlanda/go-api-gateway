package soapxml

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"

	"backend/internal/protocol"
)

const (
	Name             = "soap_xml"
	defaultNamespace = "urn:syra:soap"
	envelopeNS       = "http://schemas.xmlsoap.org/soap/envelope/"
)

type Adapter struct {
	client *http.Client
}

func NewAdapter(client *http.Client) *Adapter {
	if client == nil {
		client = http.DefaultClient
	}
	return &Adapter{client: client}
}

func (a *Adapter) Name() string {
	return Name
}

func (a *Adapter) Call(ctx context.Context, target protocol.UpstreamTarget, msg protocol.CanonicalMessage) (protocol.CanonicalMessage, error) {
	if strings.ToLower(target.Protocol) != Name {
		return protocol.CanonicalMessage{}, fmt.Errorf("unsupported upstream protocol %q", target.Protocol)
	}

	operation := firstNonEmpty(target.Metadata["soapOperation"], msg.Operation, "Request")
	namespace := firstNonEmpty(target.Metadata["soapNamespace"], defaultNamespace)
	envelope, err := buildEnvelope(operation, namespace, msg.Fields)
	if err != nil {
		return protocol.CanonicalMessage{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target.BaseURL, bytes.NewReader(envelope))
	if err != nil {
		return protocol.CanonicalMessage{}, fmt.Errorf("build soap request: %w", err)
	}
	req.Header.Set("Content-Type", `text/xml; charset="utf-8"`)
	if action := target.Metadata["soapAction"]; action != "" {
		req.Header.Set("SOAPAction", action)
	}

	resp, err := a.client.Do(req)
	if err != nil {
		return protocol.CanonicalMessage{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return protocol.CanonicalMessage{}, fmt.Errorf("read soap response: %w", err)
	}
	fields, err := extractResponseFields(body, target.Metadata["soapResponsePath"])
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
		Method:         msg.Method,
		Path:           msg.Path,
		RawQuery:       msg.RawQuery,
		Headers:        resp.Header.Clone(),
		Fields:         fields,
		Metadata:       msg.Metadata,
		StatusCode:     resp.StatusCode,
		SensitiveKeys:  msg.SensitiveKeys,
	}, nil
}

func buildEnvelope(operation string, namespace string, fields map[string]any) ([]byte, error) {
	var out bytes.Buffer
	encoder := xml.NewEncoder(&out)
	startEnvelope := xml.StartElement{
		Name: xml.Name{Local: "soapenv:Envelope"},
		Attr: []xml.Attr{
			{Name: xml.Name{Local: "xmlns:soapenv"}, Value: envelopeNS},
			{Name: xml.Name{Local: "xmlns:ns"}, Value: namespace},
		},
	}
	startBody := xml.StartElement{Name: xml.Name{Local: "soapenv:Body"}}
	startOperation := xml.StartElement{Name: xml.Name{Local: "ns:" + operation}}

	if err := encoder.EncodeToken(startEnvelope); err != nil {
		return nil, err
	}
	if err := encoder.EncodeToken(startBody); err != nil {
		return nil, err
	}
	if err := encoder.EncodeToken(startOperation); err != nil {
		return nil, err
	}
	for _, key := range sortedFieldKeys(fields) {
		startField := xml.StartElement{Name: xml.Name{Local: "ns:" + key}}
		if err := encoder.EncodeElement(fmt.Sprint(fields[key]), startField); err != nil {
			return nil, err
		}
	}
	if err := encoder.EncodeToken(startOperation.End()); err != nil {
		return nil, err
	}
	if err := encoder.EncodeToken(startBody.End()); err != nil {
		return nil, err
	}
	if err := encoder.EncodeToken(startEnvelope.End()); err != nil {
		return nil, err
	}
	if err := encoder.Flush(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func extractResponseFields(body []byte, responsePath string) (map[string]any, error) {
	decoder := xml.NewDecoder(bytes.NewReader(body))
	fields := map[string]any{}
	stack := []string{}
	targetStack := splitPath(responsePath)
	captureDepth := -1

	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("decode soap response xml: %w", err)
		}
		switch tok := token.(type) {
		case xml.StartElement:
			stack = append(stack, tok.Name.Local)
			if len(targetStack) > 0 && pathEndsWith(stack, targetStack) {
				captureDepth = len(stack)
			}
		case xml.CharData:
			if len(strings.TrimSpace(string(tok))) == 0 || len(stack) == 0 {
				continue
			}
			if captureDepth == -1 || len(stack) > captureDepth {
				fields[stack[len(stack)-1]] = strings.TrimSpace(string(tok))
			}
		case xml.EndElement:
			if captureDepth == len(stack) {
				captureDepth = -1
			}
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		}
	}
	return fields, nil
}

func splitPath(path string) []string {
	parts := []string{}
	for _, part := range strings.Split(path, ".") {
		part = strings.TrimSpace(part)
		if part != "" {
			parts = append(parts, part)
		}
	}
	return parts
}

func pathEndsWith(stack []string, suffix []string) bool {
	if len(suffix) > len(stack) {
		return false
	}
	offset := len(stack) - len(suffix)
	for idx := range suffix {
		if stack[offset+idx] != suffix[idx] {
			return false
		}
	}
	return true
}

func sortedFieldKeys(fields map[string]any) []string {
	keys := make([]string, 0, len(fields))
	for key := range fields {
		if key != "" {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return keys
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
