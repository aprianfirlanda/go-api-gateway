package transform

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"backend/internal/protocol"
)

func TestDryRunMapsFieldsStaticValuesAndFunctions(t *testing.T) {
	engine := NewEngine(
		WithClock(func() time.Time {
			return time.Date(2026, 5, 10, 12, 30, 15, 0, time.UTC)
		}),
		WithSTANGenerator(func() string {
			return "123456"
		}),
	)

	template := validTemplate()
	input := protocol.CanonicalMessage{
		TenantID:       "tenant_1",
		ConsumerID:     "consumer_1",
		APIProductID:   "product_1",
		RouteID:        "route_1",
		SourceProtocol: "rest",
		TargetProtocol: "iso8583",
		Headers: http.Header{
			"Requestid": []string{"req_1"},
		},
		Fields: map[string]any{
			"pan":        "4111111111111111",
			"amount":     10000,
			"currency":   "IDR",
			"terminalId": "ATM00101",
		},
		Metadata: map[string]any{
			"source": "mobile",
		},
	}

	output, err := engine.DryRun(context.Background(), template, DirectionRequest, input)

	require.NoError(t, err)
	require.Equal(t, "4111111111111111", output.Fields["2"])
	require.Equal(t, "000000", output.Fields["3"])
	require.Equal(t, "000000010000", output.Fields["4"])
	require.Equal(t, "0510123015", output.Fields["7"])
	require.Equal(t, "123456", output.Fields["11"])
	require.Equal(t, "ATM00101", output.Fields["41"])
	require.Equal(t, "360", output.Fields["49"])
	require.Equal(t, []string{"2"}, output.SensitiveKeys)
}

func TestDryRunSupportsMaskPanFunction(t *testing.T) {
	engine := NewEngine()
	template := validTemplate()
	template.Request.Fields = map[string]string{
		"maskedPan": "maskPan($.fields.pan)",
	}
	template.Request.Sensitive = nil

	output, err := engine.DryRun(context.Background(), template, DirectionRequest, protocol.CanonicalMessage{
		Fields:   map[string]any{"pan": "4111111111111111"},
		Headers:  http.Header{},
		Metadata: map[string]any{},
	})

	require.NoError(t, err)
	require.Equal(t, "411111******1111", output.Fields["maskedPan"])
}

func TestValidateRejectsUnsupportedExpressions(t *testing.T) {
	engine := NewEngine()
	template := validTemplate()
	template.Request.Fields["2"] = "eval($.fields.pan)"

	result := engine.Validate(template)

	require.False(t, result.Valid())
	require.Len(t, result.Errors, 1)
	require.Equal(t, "request.fields.2", result.Errors[0].Field)
	require.Contains(t, result.Errors[0].Message, "not allowed")
}

func TestValidateRejectsArbitraryScript(t *testing.T) {
	engine := NewEngine()
	template := validTemplate()
	template.Request.Fields["2"] = "$.fields.pan; panic()"

	result := engine.Validate(template)

	require.False(t, result.Valid())
	require.Equal(t, "request.fields.2", result.Errors[0].Field)
}

func TestMaskSensitiveFields(t *testing.T) {
	engine := NewEngine()

	masked := engine.MaskSensitive(protocol.CanonicalMessage{
		Fields: map[string]any{
			"2":    "4111111111111111",
			"name": "Alice",
		},
		SensitiveKeys: []string{"2", "name"},
	})

	require.Equal(t, "411111******1111", masked.Fields["2"])
	require.Equal(t, "***", masked.Fields["name"])
}

func TestResponseDryRun(t *testing.T) {
	engine := NewEngine()
	template := validTemplate()

	output, err := engine.DryRun(context.Background(), template, DirectionResponse, protocol.CanonicalMessage{
		Fields: map[string]any{
			"39": "00",
			"38": "A12345",
			"11": "123456",
		},
		Headers:  http.Header{},
		Metadata: map[string]any{},
	})

	require.NoError(t, err)
	require.Equal(t, "00", output.Fields["responseCode"])
	require.Equal(t, "A12345", output.Fields["authorizationCode"])
	require.Equal(t, "123456", output.Fields["stan"])
}

func validTemplate() Template {
	return Template{
		ID:             "template_1",
		TenantID:       "tenant_1",
		APIProductID:   "product_1",
		Name:           "card-authorization-rest-to-iso8583",
		SourceProtocol: "rest",
		TargetProtocol: "iso8583",
		Version:        1,
		Status:         StatusDraft,
		Request: Section{
			Fields: map[string]string{
				"2":  "$.fields.pan",
				"3":  "'000000'",
				"4":  "formatAmount($.fields.amount)",
				"7":  "nowMMddHHmmss()",
				"11": "generateStan()",
				"41": "$.fields.terminalId",
				"49": "currencyNumeric($.fields.currency)",
			},
			Sensitive: []string{"2"},
		},
		Response: Section{
			Fields: map[string]string{
				"responseCode":      "$.fields.39",
				"authorizationCode": "$.fields.38",
				"stan":              "$.fields.11",
			},
		},
	}
}
