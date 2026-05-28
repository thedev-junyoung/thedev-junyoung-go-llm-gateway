package provider_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/thedev-junyoung/thedev-junyoung-go-llm-gateway/pkg/provider"
)

func TestNewProviderError_SetsFields(t *testing.T) {
	t.Parallel()

	wrapped := errors.New("io: connection reset")
	err := provider.NewProviderError("openai", provider.ErrorTypeRateLimit, 429, true, "rate limit exceeded", wrapped)

	if got, want := err.Type, provider.ErrorTypeRateLimit; got != want {
		t.Errorf("Type = %q, want %q", got, want)
	}
	if got, want := err.Vendor(), "openai"; got != want {
		t.Errorf("Vendor() = %q, want %q", got, want)
	}
	if got, want := err.StatusCode, 429; got != want {
		t.Errorf("StatusCode = %d, want %d", got, want)
	}
	if !err.Retriable {
		t.Error("Retriable = false, want true")
	}
	if got, want := err.VendorMessage(), "rate limit exceeded"; got != want {
		t.Errorf("VendorMessage() = %q, want %q", got, want)
	}
	if !errors.Is(err, wrapped) {
		t.Error("errors.Is(err, wrapped) = false, want true (Unwrap chain broken)")
	}
}

func TestProviderError_ErrorString_DoesNotLeakVendorMessage(t *testing.T) {
	t.Parallel()

	const secret = "internal-prompt-secret-9f0c2"
	err := provider.NewProviderError("anthropic", provider.ErrorTypeAuth, 401, false, secret, nil)

	if strings.Contains(err.Error(), secret) {
		t.Errorf("Error() leaked vendor message %q in %q", secret, err.Error())
	}
	// The sanitized form still needs to be useful for debugging.
	for _, want := range []string{"anthropic", "auth", "401"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("Error() = %q, missing %q", err.Error(), want)
		}
	}
}

func TestProviderError_Is_RouterCriticalSentinels(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		errType  provider.ErrorType
		sentinel error
		want     bool
	}{
		{"rate_limit matches ErrRateLimited", provider.ErrorTypeRateLimit, provider.ErrRateLimited, true},
		{"overloaded matches ErrOverloaded", provider.ErrorTypeOverloaded, provider.ErrOverloaded, true},
		{"auth matches ErrAuthFailed", provider.ErrorTypeAuth, provider.ErrAuthFailed, true},
		{"timeout matches ErrTimeout", provider.ErrorTypeTimeout, provider.ErrTimeout, true},

		// Non-router-critical categories deliberately don't match sentinels —
		// they're reached via errors.As + Type inspection.
		{"permission does not match ErrAuthFailed", provider.ErrorTypePermission, provider.ErrAuthFailed, false},
		{"invalid_input does not match any sentinel", provider.ErrorTypeInvalidInput, provider.ErrRateLimited, false},
		{"server does not match ErrOverloaded", provider.ErrorTypeServer, provider.ErrOverloaded, false},
		{"unknown does not match any sentinel", provider.ErrorTypeUnknown, provider.ErrTimeout, false},
		{"not_found does not match any sentinel", provider.ErrorTypeNotFound, provider.ErrRateLimited, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := provider.NewProviderError("openai", tc.errType, 0, false, "", nil)
			if got := errors.Is(err, tc.sentinel); got != tc.want {
				t.Errorf("errors.Is(err{%s}, %v) = %v, want %v", tc.errType, tc.sentinel, got, tc.want)
			}
		})
	}
}

func TestProviderError_WithRetryAfter_Chains(t *testing.T) {
	t.Parallel()

	const wait = 5 * time.Second
	err := provider.NewProviderError("openai", provider.ErrorTypeRateLimit, 429, true, "", nil).
		WithRetryAfter(wait)

	if err.RetryAfter == nil {
		t.Fatal("RetryAfter = nil, want non-nil after WithRetryAfter")
	}
	if got := *err.RetryAfter; got != wait {
		t.Errorf("RetryAfter = %v, want %v", got, wait)
	}
}

func TestProviderError_RetryAfter_NilWhenAbsent(t *testing.T) {
	t.Parallel()

	err := provider.NewProviderError("openai", provider.ErrorTypeServer, 500, true, "", nil)
	if err.RetryAfter != nil {
		t.Errorf("RetryAfter = %v, want nil (no WithRetryAfter call)", *err.RetryAfter)
	}
}

