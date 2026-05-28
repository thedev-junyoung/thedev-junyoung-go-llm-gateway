package gateway

import (
	"context"

	"github.com/thedev-junyoung/thedev-junyoung-go-llm-gateway/internal/requestctx"
)

// WithRequestID attaches a request id to ctx. Middleware or callers use this
// to correlate logs, metrics, and provider failover traces across the
// internal packages without each package having to define its own key.
func WithRequestID(ctx context.Context, id string) context.Context {
	return requestctx.With(ctx, id)
}

// RequestIDFromContext returns the request id previously set with
// WithRequestID, or "" if none was set. The internal/requestctx package is
// not importable from outside this module, so callers (loggers, tracers)
// reach the value through this accessor.
func RequestIDFromContext(ctx context.Context) string {
	return requestctx.From(ctx)
}
