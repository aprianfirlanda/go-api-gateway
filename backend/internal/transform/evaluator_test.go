package transform

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"backend/internal/protocol"
)

func TestEvaluatorBuiltInFunctions(t *testing.T) {
	evaluator := NewEvaluator(
		func() time.Time {
			return time.Date(2026, 5, 10, 12, 30, 15, 0, time.UTC)
		},
		func() string {
			return "654321"
		},
	)
	msg := protocol.CanonicalMessage{
		Fields: map[string]any{
			"amount":   10000,
			"currency": "USD",
			"pan":      "4111111111111111",
		},
		Headers:  http.Header{},
		Metadata: map[string]any{},
	}

	tests := map[string]any{
		"formatAmount($.fields.amount)":      "000000010000",
		"currencyNumeric($.fields.currency)": "840",
		"nowMMddHHmmss()":                    "0510123015",
		"generateStan()":                     "654321",
		"maskPan($.fields.pan)":              "411111******1111",
	}

	for expression, want := range tests {
		t.Run(expression, func(t *testing.T) {
			got, err := evaluator.Evaluate(expression, msg)
			require.NoError(t, err)
			require.Equal(t, want, got)
		})
	}
}

func TestEvaluatorReadsPathsAndLiterals(t *testing.T) {
	evaluator := NewEvaluator(time.Now, func() string { return "000001" })
	msg := protocol.CanonicalMessage{
		Fields:   map[string]any{"amount": 10000},
		Headers:  http.Header{"Requestid": []string{"req_1"}},
		Metadata: map[string]any{"source": "mobile"},
	}

	got, err := evaluator.Evaluate("$.fields.amount", msg)
	require.NoError(t, err)
	require.Equal(t, 10000, got)

	got, err = evaluator.Evaluate("$.headers.requestId", msg)
	require.NoError(t, err)
	require.Equal(t, "req_1", got)

	got, err = evaluator.Evaluate("$.metadata.source", msg)
	require.NoError(t, err)
	require.Equal(t, "mobile", got)

	got, err = evaluator.Evaluate("'static_value'", msg)
	require.NoError(t, err)
	require.Equal(t, "static_value", got)
}
