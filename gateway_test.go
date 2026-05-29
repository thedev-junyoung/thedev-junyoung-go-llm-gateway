package gateway_test

import (
	"context"
	"errors"
	"testing"

	gateway "github.com/thedev-junyoung/thedev-junyoung-go-llm-gateway"
	"github.com/thedev-junyoung/thedev-junyoung-go-llm-gateway/pkg/provider"
)

// fakeProvider exercises Gateway without dragging in the real OpenAI or
// Anthropic adapter packages. ChatFn lets each test inject the response.
type fakeProvider struct {
	name   string
	models map[string]struct{}
	ChatFn func(ctx context.Context, req provider.ChatRequest) (provider.ChatResponse, error)
}

func newFake(name string, models []string, chat func(context.Context, provider.ChatRequest) (provider.ChatResponse, error)) *fakeProvider {
	m := make(map[string]struct{}, len(models))
	for _, model := range models {
		m[model] = struct{}{}
	}
	return &fakeProvider{name: name, models: m, ChatFn: chat}
}

func (f *fakeProvider) Name() string                    { return f.name }
func (f *fakeProvider) SupportsModel(model string) bool { _, ok := f.models[model]; return ok }
func (f *fakeProvider) Chat(ctx context.Context, req provider.ChatRequest) (provider.ChatResponse, error) {
	return f.ChatFn(ctx, req)
}

var _ provider.Provider = (*fakeProvider)(nil)

func TestNew_EmptyProviders_ReturnsErrNoProviders(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		cfg  gateway.Config
	}{
		{"nil slice", gateway.Config{Providers: nil}},
		{"empty slice", gateway.Config{Providers: []provider.Provider{}}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			gw, err := gateway.New(tc.cfg)
			if !errors.Is(err, gateway.ErrNoProviders) {
				t.Errorf("err = %v, want errors.Is == ErrNoProviders", err)
			}
			if gw != nil {
				t.Error("Gateway is not nil on error")
			}
		})
	}
}

func TestNew_WithProviders_Succeeds(t *testing.T) {
	t.Parallel()

	p := newFake("openai", []string{"gpt-4o"}, nil)
	gw, err := gateway.New(gateway.Config{Providers: []provider.Provider{p}})
	if err != nil {
		t.Fatalf("New err = %v, want nil", err)
	}
	if gw == nil {
		t.Fatal("Gateway is nil")
	}
}

func TestChat_RoutesToFirstSupportingProvider(t *testing.T) {
	t.Parallel()

	called := ""
	openai := newFake("openai", []string{"gpt-4o"},
		func(_ context.Context, _ provider.ChatRequest) (provider.ChatResponse, error) {
			called = "openai"
			return provider.ChatResponse{Content: "from openai", FinishReason: provider.FinishStop}, nil
		})
	anthropic := newFake("anthropic", []string{"claude-opus-4-7"},
		func(_ context.Context, _ provider.ChatRequest) (provider.ChatResponse, error) {
			called = "anthropic"
			return provider.ChatResponse{Content: "from anthropic", FinishReason: provider.FinishStop}, nil
		})

	gw, err := gateway.New(gateway.Config{Providers: []provider.Provider{openai, anthropic}})
	if err != nil {
		t.Fatalf("New err = %v", err)
	}

	resp, err := gw.Chat(context.Background(), provider.ChatRequest{
		Model:    "claude-opus-4-7",
		Messages: []provider.Message{{Role: provider.RoleUser, Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("Chat err = %v", err)
	}
	if called != "anthropic" {
		t.Errorf("routed to %q, want %q", called, "anthropic")
	}
	if resp.Content != "from anthropic" {
		t.Errorf("Content = %q, want %q", resp.Content, "from anthropic")
	}
}

func TestChat_UnsupportedModel_ReturnsGatewayInvalidInput(t *testing.T) {
	t.Parallel()

	p := newFake("openai", []string{"gpt-4o"}, nil)
	gw, _ := gateway.New(gateway.Config{Providers: []provider.Provider{p}})

	_, err := gw.Chat(context.Background(), provider.ChatRequest{
		Model:    "gemini-2.0-pro",
		Messages: []provider.Message{{Role: provider.RoleUser, Content: "hi"}},
	})

	var pe *provider.ProviderError
	if !errors.As(err, &pe) {
		t.Fatalf("err is not *ProviderError: %T", err)
	}
	if pe.Type != provider.ErrorTypeInvalidInput {
		t.Errorf("Type = %q, want %q", pe.Type, provider.ErrorTypeInvalidInput)
	}
	if pe.Vendor() != "gateway" {
		t.Errorf("Vendor() = %q, want %q (routing origin)", pe.Vendor(), "gateway")
	}
}

func TestChat_ProviderError_PropagatesVerbatim(t *testing.T) {
	t.Parallel()

	// Adapter returns a rate-limit error. v0.1 has no failover, so the
	// caller sees the vendor's error directly — sentinel still works.
	rateLimited := provider.NewProviderError("openai", provider.ErrorTypeRateLimit, 429, true, "slow down", nil)
	p := newFake("openai", []string{"gpt-4o"},
		func(_ context.Context, _ provider.ChatRequest) (provider.ChatResponse, error) {
			return provider.ChatResponse{}, rateLimited
		})

	gw, _ := gateway.New(gateway.Config{Providers: []provider.Provider{p}})
	_, err := gw.Chat(context.Background(), provider.ChatRequest{
		Model:    "gpt-4o",
		Messages: []provider.Message{{Role: provider.RoleUser, Content: "hi"}},
	})

	if !errors.Is(err, provider.ErrRateLimited) {
		t.Errorf("errors.Is(err, ErrRateLimited) = false, want true (sentinel must survive passthrough)")
	}
}
