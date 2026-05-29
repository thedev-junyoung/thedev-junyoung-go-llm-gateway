// Package router picks which Provider serves a given model and prepares the
// fallback chain for the gateway's failover loop. Per ADR-003, routing is
// "first-match by SupportsModel against Config.Providers order" — there is
// no separate model→provider map, and the slice order IS the priority.
//
// This package is the v0.1 routing primitive. The failover orchestration
// (retry / backoff / in-flight tracking) lands in a follow-up PR and ADR-004.
package router

import (
	"fmt"

	"github.com/thedev-junyoung/thedev-junyoung-go-llm-gateway/pkg/provider"
)

// gatewayVendor is the Vendor() value on routing-layer errors. Using a
// stable string lets call sites distinguish a routing failure (model
// unsupported by any registered provider) from a vendor-side rejection.
const gatewayVendor = "gateway"

// Pick returns the first provider in `providers` whose SupportsModel(model)
// returns true. Argument order IS the priority — gateway.Config.Providers
// is passed through verbatim.
//
// When no provider supports the model, Pick returns a *provider.ProviderError
// with Type == ErrorTypeInvalidInput and Vendor() == "gateway".
func Pick(providers []provider.Provider, model string) (provider.Provider, error) {
	for _, p := range providers {
		if p.SupportsModel(model) {
			return p, nil
		}
	}
	return nil, unsupportedModelError(model)
}

// PickWithFallbacks returns the primary provider plus the rest of the
// providers that support `model`, in Config order. The failover loop calls
// this once at request entry and tries primary first, then iterates
// fallbacks in order on retriable failures.
//
// Per ADR-003 Q5, providers that do NOT support the requested model are
// excluded from BOTH lists — the gateway never auto-translates the model
// across vendors. When no provider supports the model, returns the same
// error as Pick.
func PickWithFallbacks(providers []provider.Provider, model string) (primary provider.Provider, fallbacks []provider.Provider, err error) {
	supporting := make([]provider.Provider, 0, len(providers))
	for _, p := range providers {
		if p.SupportsModel(model) {
			supporting = append(supporting, p)
		}
	}
	if len(supporting) == 0 {
		return nil, nil, unsupportedModelError(model)
	}
	return supporting[0], supporting[1:], nil
}

func unsupportedModelError(model string) *provider.ProviderError {
	return provider.NewProviderError(
		gatewayVendor,
		provider.ErrorTypeInvalidInput,
		0,
		false,
		fmt.Sprintf("no registered provider supports model %q", model),
		nil,
	)
}
