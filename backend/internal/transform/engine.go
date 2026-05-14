package transform

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"sort"
	"time"

	"backend/internal/protocol"
)

type Engine struct {
	now         func() time.Time
	stan        func() string
	expressions *Evaluator
}

type Option func(*Engine)

func NewEngine(opts ...Option) *Engine {
	engine := &Engine{
		now: func() time.Time {
			return time.Now().UTC()
		},
		stan: func() string {
			return fmt.Sprintf("%06d", rand.Intn(1000000))
		},
	}
	engine.expressions = NewEvaluator(engine.now, engine.stan)

	for _, opt := range opts {
		opt(engine)
	}
	engine.expressions = NewEvaluator(engine.now, engine.stan)

	return engine
}

func WithClock(now func() time.Time) Option {
	return func(engine *Engine) {
		if now != nil {
			engine.now = now
		}
	}
}

func WithSTANGenerator(stan func() string) Option {
	return func(engine *Engine) {
		if stan != nil {
			engine.stan = stan
		}
	}
}

func (e *Engine) Validate(template Template) ValidationResult {
	var result ValidationResult

	if template.ID == "" {
		result.Errors = append(result.Errors, ValidationError{Field: "id", Message: "id is required"})
	}
	if template.TenantID == "" {
		result.Errors = append(result.Errors, ValidationError{Field: "tenantId", Message: "tenantId is required"})
	}
	if template.SourceProtocol == "" {
		result.Errors = append(result.Errors, ValidationError{Field: "sourceProtocol", Message: "sourceProtocol is required"})
	}
	if template.TargetProtocol == "" {
		result.Errors = append(result.Errors, ValidationError{Field: "targetProtocol", Message: "targetProtocol is required"})
	}
	if template.Version <= 0 {
		result.Errors = append(result.Errors, ValidationError{Field: "version", Message: "version must be greater than zero"})
	}
	switch template.Status {
	case StatusDraft, StatusPublished, StatusArchived, StatusDisabled:
	default:
		result.Errors = append(result.Errors, ValidationError{Field: "status", Message: "status is invalid"})
	}

	result.Errors = append(result.Errors, e.validateSection("request", template.Request)...)
	result.Errors = append(result.Errors, e.validateSection("response", template.Response)...)

	return result
}

func (e *Engine) DryRun(ctx context.Context, template Template, direction Direction, input protocol.CanonicalMessage) (protocol.CanonicalMessage, error) {
	if err := ctx.Err(); err != nil {
		return protocol.CanonicalMessage{}, err
	}

	section, err := sectionForDirection(template, direction)
	if err != nil {
		return protocol.CanonicalMessage{}, err
	}

	if result := e.Validate(template); !result.Valid() {
		return protocol.CanonicalMessage{}, fmt.Errorf("template validation failed: %s", result.Errors[0].Message)
	}

	output := protocol.CanonicalMessage{
		TenantID:       input.TenantID,
		ConsumerID:     input.ConsumerID,
		CredentialID:   input.CredentialID,
		APIProductID:   input.APIProductID,
		RouteID:        input.RouteID,
		SourceProtocol: template.SourceProtocol,
		TargetProtocol: template.TargetProtocol,
		Operation:      input.Operation,
		Method:         input.Method,
		Path:           input.Path,
		RawQuery:       input.RawQuery,
		Headers:        cloneHeader(input.Headers),
		Fields:         map[string]any{},
		Metadata:       cloneMap(input.Metadata),
		Body:           input.Body,
		StatusCode:     input.StatusCode,
		SensitiveKeys:  append([]string(nil), section.Sensitive...),
	}

	for _, field := range sortedKeys(section.Fields) {
		value, err := e.expressions.Evaluate(section.Fields[field], input)
		if err != nil {
			return protocol.CanonicalMessage{}, fmt.Errorf("evaluate field %s: %w", field, err)
		}
		output.Fields[field] = value
	}

	return output, nil
}

func (e *Engine) MaskSensitive(msg protocol.CanonicalMessage) protocol.CanonicalMessage {
	masked := msg
	masked.Headers = cloneHeader(msg.Headers)
	masked.Metadata = cloneMap(msg.Metadata)
	masked.Fields = cloneMap(msg.Fields)
	masked.SensitiveKeys = append([]string(nil), msg.SensitiveKeys...)

	for _, key := range msg.SensitiveKeys {
		if value, ok := masked.Fields[key]; ok {
			masked.Fields[key] = maskSensitiveValue(value)
		}
	}

	return masked
}

func (e *Engine) validateSection(prefix string, section Section) []ValidationError {
	var errors []ValidationError

	for field, expression := range section.Fields {
		if field == "" {
			errors = append(errors, ValidationError{Field: prefix + ".fields", Message: "field name is required"})
			continue
		}
		if err := e.expressions.Validate(expression); err != nil {
			errors = append(errors, ValidationError{
				Field:   prefix + ".fields." + field,
				Message: err.Error(),
			})
		}
	}

	return errors
}

func sectionForDirection(template Template, direction Direction) (Section, error) {
	switch direction {
	case DirectionRequest:
		return template.Request, nil
	case DirectionResponse:
		return template.Response, nil
	default:
		return Section{}, fmt.Errorf("unknown transform direction %q", direction)
	}
}

func cloneHeader(in http.Header) http.Header {
	if in == nil {
		return http.Header{}
	}
	return in.Clone()
}

func cloneMap(in map[string]any) map[string]any {
	out := map[string]any{}
	for key, value := range in {
		out[key] = value
	}
	return out
}

func sortedKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
