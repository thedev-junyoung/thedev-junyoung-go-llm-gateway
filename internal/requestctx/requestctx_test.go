package requestctx_test

import (
	"context"
	"testing"

	"github.com/thedev-junyoung/thedev-junyoung-go-llm-gateway/internal/requestctx"
)

func TestWithFrom_RoundTrip(t *testing.T) {
	t.Parallel()

	const id = "req_01HXYZ"
	ctx := requestctx.With(context.Background(), id)

	if got := requestctx.From(ctx); got != id {
		t.Errorf("From(ctx) = %q, want %q", got, id)
	}
}

func TestFrom_EmptyWhenAbsent(t *testing.T) {
	t.Parallel()

	if got := requestctx.From(context.Background()); got != "" {
		t.Errorf("From(empty ctx) = %q, want \"\"", got)
	}
}

func TestFrom_DoesNotCollideWithStringKey(t *testing.T) {
	t.Parallel()

	// A package that uses a string key with the same literal name must NOT
	// be reachable through requestctx.From — this guards the contextKey struct
	// pattern from regressing back to a string key.
	type extKey string
	ctx := context.WithValue(context.Background(), extKey("requestID"), "spoofed")

	if got := requestctx.From(ctx); got != "" {
		t.Errorf("From(ctx with foreign key) = %q, want \"\" (key collision)", got)
	}
}
