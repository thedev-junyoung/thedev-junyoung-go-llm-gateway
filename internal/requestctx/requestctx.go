// Package requestctx carries the gateway request id through context.Context.
// It lives in internal/ so every pkg/* (router, ratelimit, metrics, logging,
// provider) can read it without violating the "pkg/* must not import pkg/*"
// boundary rule from docs/design/architecture.md. The composition root
// (package gateway) re-exposes With/From for external callers.
package requestctx

import "context"

// contextKey is unexported and unique per package — using a struct type
// (rather than a string) prevents accidental key collisions across packages.
type contextKey struct{ name string }

var requestIDKey = contextKey{name: "requestID"}

// With returns a context carrying the given request id.
func With(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey, id)
}

// From returns the request id stored in ctx, or "" if absent.
func From(ctx context.Context) string {
	if v, ok := ctx.Value(requestIDKey).(string); ok {
		return v
	}
	return ""
}
