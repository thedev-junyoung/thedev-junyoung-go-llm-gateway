package gateway

import (
	"context"
	"errors"

	"github.com/thedev-junyoung/thedev-junyoung-go-llm-gateway/pkg/provider"
	"github.com/thedev-junyoung/thedev-junyoung-go-llm-gateway/pkg/router"
)

// Config configures a Gateway. Only Providers is required in v0.1.
//
// Failover policy, rate limiting, metrics, and logging fields will be added
// in subsequent ADRs (ADR-004 onward) as non-breaking struct additions —
// today this single-provider passthrough is the floor.
type Config struct {
	// Providers is the ordered set of vendor adapters. The slice order IS the
	// priority — for a given model, the first provider whose SupportsModel
	// returns true serves the request (see ADR-003).
	Providers []provider.Provider
}

// Gateway is the composition root: it owns the providers and dispatches
// Chat through the routing layer. Construct with New, never via struct
// literal — future ADRs may add invariants (initialized rate limiter,
// metrics recorder, etc.) that the constructor enforces.
type Gateway struct {
	providers []provider.Provider
}

// ErrNoProviders is returned by New when Config.Providers is empty.
var ErrNoProviders = errors.New("gateway: Config.Providers must contain at least one provider")

// New validates the config and returns a ready Gateway.
func New(cfg Config) (*Gateway, error) {
	if len(cfg.Providers) == 0 {
		return nil, ErrNoProviders
	}
	return &Gateway{providers: cfg.Providers}, nil
}

// Chat routes the request to the first provider that supports req.Model
// (per ADR-003) and returns its response.
//
// v0.1 has no failover: a provider error propagates verbatim. The router
// already returns *provider.ProviderError with Vendor()=="gateway" for the
// "no provider supports this model" case, so call sites can distinguish
// routing failures from vendor-side rejections without parsing strings.
func (g *Gateway) Chat(ctx context.Context, req provider.ChatRequest) (provider.ChatResponse, error) {
	p, err := router.Pick(g.providers, req.Model)
	if err != nil {
		return provider.ChatResponse{}, err
	}
	return p.Chat(ctx, req)
}
