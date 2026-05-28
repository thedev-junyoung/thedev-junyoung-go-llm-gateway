package gateway_test

import (
	"context"
	"testing"

	gateway "github.com/thedev-junyoung/thedev-junyoung-go-llm-gateway"
)

func TestWithRequestID_RoundTrip(t *testing.T) {
	t.Parallel()

	const id = "req_external"
	ctx := gateway.WithRequestID(context.Background(), id)

	if got := gateway.RequestIDFromContext(ctx); got != id {
		t.Errorf("RequestIDFromContext = %q, want %q", got, id)
	}
}

func TestRequestIDFromContext_EmptyByDefault(t *testing.T) {
	t.Parallel()

	if got := gateway.RequestIDFromContext(context.Background()); got != "" {
		t.Errorf("RequestIDFromContext(empty) = %q, want \"\"", got)
	}
}
