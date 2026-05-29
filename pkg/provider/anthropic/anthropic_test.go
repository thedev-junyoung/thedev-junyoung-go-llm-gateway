package anthropic_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/thedev-junyoung/thedev-junyoung-go-llm-gateway/pkg/provider"
	"github.com/thedev-junyoung/thedev-junyoung-go-llm-gateway/pkg/provider/anthropic"
)

func TestClient_Name(t *testing.T) {
	t.Parallel()
	c := anthropic.New("sk-ant-test")
	if got, want := c.Name(), "anthropic"; got != want {
		t.Errorf("Name() = %q, want %q", got, want)
	}
}

func TestClient_SupportsModel(t *testing.T) {
	t.Parallel()

	c := anthropic.New("sk-ant-test")
	cases := []struct {
		model string
		want  bool
	}{
		{"claude-opus-4-7", true},
		{"claude-sonnet-4-6", true},
		{"claude-haiku-4-5-20251001", true},
		{"gpt-4o", false},
		{"unknown", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := c.SupportsModel(tc.model); got != tc.want {
			t.Errorf("SupportsModel(%q) = %v, want %v", tc.model, got, tc.want)
		}
	}
}

func TestChat_HappyPath_SystemTopLevel(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.Path, "/messages"; got != want {
			t.Errorf("path = %q, want %q", got, want)
		}
		if got, want := r.Header.Get("x-api-key"), "sk-ant-test"; got != want {
			t.Errorf("x-api-key = %q, want %q", got, want)
		}
		if r.Header.Get("anthropic-version") == "" {
			t.Error("anthropic-version header missing")
		}

		body, _ := io.ReadAll(r.Body)
		// System must be a top-level field, not in messages array.
		if !strings.Contains(string(body), `"system":"sys"`) {
			t.Errorf("system top-level field missing, body=%s", body)
		}
		if strings.Contains(string(body), `"role":"system"`) {
			t.Errorf("system MUST NOT appear as a message role; body=%s", body)
		}
		// max_tokens must be present and non-zero (Anthropic requires it).
		if !strings.Contains(string(body), `"max_tokens":1024`) {
			t.Errorf("max_tokens=1024 missing from body=%s", body)
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"content": []map[string]any{
				{"type": "text", "text": "hello back"},
			},
			"stop_reason": "end_turn",
			"usage": map[string]int{
				"input_tokens":  12,
				"output_tokens": 3,
			},
		})
	}))
	defer server.Close()

	c := anthropic.New("sk-ant-test", anthropic.WithBaseURL(server.URL))
	maxTok := 1024
	resp, err := c.Chat(context.Background(), provider.ChatRequest{
		Model:     "claude-opus-4-7",
		System:    "sys",
		Messages:  []provider.Message{{Role: provider.RoleUser, Content: "hi"}},
		MaxTokens: &maxTok,
	})
	if err != nil {
		t.Fatalf("Chat() err = %v", err)
	}
	if resp.Content != "hello back" {
		t.Errorf("Content = %q, want %q", resp.Content, "hello back")
	}
	if resp.FinishReason != provider.FinishStop {
		t.Errorf("FinishReason = %q, want %q", resp.FinishReason, provider.FinishStop)
	}
	if resp.Usage.InputTokens != 12 || resp.Usage.OutputTokens != 3 {
		t.Errorf("Usage = %+v, want {12 3}", resp.Usage)
	}
}

func TestChat_MultiTextBlockJoin(t *testing.T) {
	t.Parallel()

	// Anthropic can return multiple text blocks (extended thinking, etc.);
	// contract requires "\n" join so two providers yield byte-identical output.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"content": []map[string]any{
				{"type": "text", "text": "A"},
				{"type": "tool_use", "id": "t1", "name": "x"},
				{"type": "text", "text": "B"},
			},
			"stop_reason": "end_turn",
			"usage":       map[string]int{},
		})
	}))
	defer server.Close()

	c := anthropic.New("sk-ant-test", anthropic.WithBaseURL(server.URL),
		anthropic.WithDefaultMaxTokens(1024))
	resp, err := c.Chat(context.Background(), provider.ChatRequest{
		Model:    "claude-opus-4-7",
		Messages: []provider.Message{{Role: provider.RoleUser, Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("Chat() err = %v", err)
	}
	if resp.Content != "A\nB" {
		t.Errorf("Content = %q, want %q (text blocks joined with LF, non-text dropped)", resp.Content, "A\nB")
	}
}

func TestChat_DefaultMaxTokensFallback(t *testing.T) {
	t.Parallel()

	var captured string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		captured = string(body)
		_, _ = w.Write([]byte(`{"content":[{"type":"text","text":"ok"}],"stop_reason":"end_turn","usage":{}}`))
	}))
	defer server.Close()

	c := anthropic.New("sk-ant-test", anthropic.WithBaseURL(server.URL),
		anthropic.WithDefaultMaxTokens(4096))
	_, err := c.Chat(context.Background(), provider.ChatRequest{
		Model:    "claude-opus-4-7",
		Messages: []provider.Message{{Role: provider.RoleUser, Content: "hi"}},
		// MaxTokens nil — fallback should kick in.
	})
	if err != nil {
		t.Fatalf("Chat() err = %v", err)
	}
	if !strings.Contains(captured, `"max_tokens":4096`) {
		t.Errorf("max_tokens fallback missing from body=%s", captured)
	}
}

func TestChat_NilMaxTokens_NoFallback_InvalidInput(t *testing.T) {
	t.Parallel()

	// No httptest server — Chat() MUST reject before any HTTP call.
	c := anthropic.New("sk-ant-test", anthropic.WithBaseURL("http://0.0.0.0:0"))
	_, err := c.Chat(context.Background(), provider.ChatRequest{
		Model:    "claude-opus-4-7",
		Messages: []provider.Message{{Role: provider.RoleUser, Content: "hi"}},
	})

	var pe *provider.ProviderError
	if !errors.As(err, &pe) {
		t.Fatalf("err is not *ProviderError: %T", err)
	}
	if pe.Type != provider.ErrorTypeInvalidInput {
		t.Errorf("Type = %q, want %q", pe.Type, provider.ErrorTypeInvalidInput)
	}
	if pe.StatusCode != 0 {
		t.Errorf("StatusCode = %d, want 0 (no HTTP call)", pe.StatusCode)
	}
}

func TestChat_ZeroOrNegativeMaxTokens_InvalidInput(t *testing.T) {
	t.Parallel()

	c := anthropic.New("sk-ant-test", anthropic.WithBaseURL("http://0.0.0.0:0"))
	zero := 0
	_, err := c.Chat(context.Background(), provider.ChatRequest{
		Model:     "claude-opus-4-7",
		Messages:  []provider.Message{{Role: provider.RoleUser, Content: "hi"}},
		MaxTokens: &zero,
	})

	var pe *provider.ProviderError
	if !errors.As(err, &pe) {
		t.Fatalf("err is not *ProviderError: %T", err)
	}
	if pe.Type != provider.ErrorTypeInvalidInput {
		t.Errorf("Type = %q, want %q (MaxTokens=0 is invalid)", pe.Type, provider.ErrorTypeInvalidInput)
	}
}

func TestChat_UnsupportedModel(t *testing.T) {
	t.Parallel()

	c := anthropic.New("sk-ant-test")
	_, err := c.Chat(context.Background(), provider.ChatRequest{
		Model:    "gpt-4o",
		Messages: []provider.Message{{Role: provider.RoleUser, Content: "hi"}},
	})

	var pe *provider.ProviderError
	if !errors.As(err, &pe) {
		t.Fatalf("err is not *ProviderError: %T", err)
	}
	if pe.Type != provider.ErrorTypeInvalidInput {
		t.Errorf("Type = %q, want %q", pe.Type, provider.ErrorTypeInvalidInput)
	}
}

func TestChat_ErrorMapping(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		status    int
		errType   string
		wantType  provider.ErrorType
		wantRetry bool
		sentinel  error
	}{
		{"429 rate_limit", 429, "rate_limit_error", provider.ErrorTypeRateLimit, true, provider.ErrRateLimited},
		{"401 auth", 401, "authentication_error", provider.ErrorTypeAuth, false, provider.ErrAuthFailed},
		{"403 permission", 403, "permission_error", provider.ErrorTypePermission, false, nil},
		{"404 not_found", 404, "not_found_error", provider.ErrorTypeNotFound, false, nil},
		{"400 invalid", 400, "invalid_request_error", provider.ErrorTypeInvalidInput, false, nil},
		{"503 overloaded_error → Overloaded", 503, "overloaded_error", provider.ErrorTypeOverloaded, true, provider.ErrOverloaded},
		{"500 api_error → Server", 500, "api_error", provider.ErrorTypeServer, true, nil},
		{"418 unknown", 418, "unknown_error", provider.ErrorTypeUnknown, false, nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(`{"type":"error","error":{"type":"` + tc.errType + `","message":"boom"}}`))
			}))
			defer server.Close()

			maxTok := 1024
			c := anthropic.New("sk-ant-test", anthropic.WithBaseURL(server.URL))
			_, err := c.Chat(context.Background(), provider.ChatRequest{
				Model:     "claude-opus-4-7",
				Messages:  []provider.Message{{Role: provider.RoleUser, Content: "hi"}},
				MaxTokens: &maxTok,
			})

			var pe *provider.ProviderError
			if !errors.As(err, &pe) {
				t.Fatalf("err is not *ProviderError: %T", err)
			}
			if pe.Type != tc.wantType {
				t.Errorf("Type = %q, want %q", pe.Type, tc.wantType)
			}
			if pe.Retriable != tc.wantRetry {
				t.Errorf("Retriable = %v, want %v", pe.Retriable, tc.wantRetry)
			}
			if tc.sentinel != nil && !errors.Is(err, tc.sentinel) {
				t.Errorf("errors.Is(err, %v) = false, want true", tc.sentinel)
			}
		})
	}
}

func TestChat_RetryAfterHeader(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "11")
		w.WriteHeader(429)
		_, _ = w.Write([]byte(`{"type":"error","error":{"type":"rate_limit_error","message":"slow"}}`))
	}))
	defer server.Close()

	maxTok := 1024
	c := anthropic.New("sk-ant-test", anthropic.WithBaseURL(server.URL))
	_, err := c.Chat(context.Background(), provider.ChatRequest{
		Model:     "claude-opus-4-7",
		Messages:  []provider.Message{{Role: provider.RoleUser, Content: "hi"}},
		MaxTokens: &maxTok,
	})

	var pe *provider.ProviderError
	if !errors.As(err, &pe) {
		t.Fatalf("err is not *ProviderError: %T", err)
	}
	if pe.RetryAfter == nil || *pe.RetryAfter != 11*time.Second {
		t.Errorf("RetryAfter = %v, want 11s", pe.RetryAfter)
	}
}

func TestChat_ContextCanceled(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))
	defer server.Close()

	maxTok := 1024
	c := anthropic.New("sk-ant-test", anthropic.WithBaseURL(server.URL))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := c.Chat(ctx, provider.ChatRequest{
		Model:     "claude-opus-4-7",
		Messages:  []provider.Message{{Role: provider.RoleUser, Content: "hi"}},
		MaxTokens: &maxTok,
	})

	var pe *provider.ProviderError
	if !errors.As(err, &pe) {
		t.Fatalf("err is not *ProviderError: %T", err)
	}
	if pe.Type != provider.ErrorTypeTimeout {
		t.Errorf("Type = %q, want %q", pe.Type, provider.ErrorTypeTimeout)
	}
	if !errors.Is(err, provider.ErrTimeout) {
		t.Error("errors.Is(err, ErrTimeout) = false, want true")
	}
}

func TestChat_StopReasonMapping(t *testing.T) {
	t.Parallel()

	cases := []struct {
		raw  string
		want provider.FinishReason
	}{
		{"end_turn", provider.FinishStop},
		{"max_tokens", provider.FinishLength},
		{"stop_sequence", provider.FinishStopSequence},
		{"tool_use", provider.FinishToolUse},
		{"some_future_value", provider.FinishUnknown},
	}

	for _, tc := range cases {
		t.Run(tc.raw, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(`{"content":[{"type":"text","text":"x"}],"stop_reason":"` + tc.raw + `","usage":{}}`))
			}))
			defer server.Close()

			maxTok := 1024
			c := anthropic.New("sk-ant-test", anthropic.WithBaseURL(server.URL))
			resp, err := c.Chat(context.Background(), provider.ChatRequest{
				Model:     "claude-opus-4-7",
				Messages:  []provider.Message{{Role: provider.RoleUser, Content: "hi"}},
				MaxTokens: &maxTok,
			})
			if err != nil {
				t.Fatalf("Chat() err = %v", err)
			}
			if resp.FinishReason != tc.want {
				t.Errorf("FinishReason for %q = %q, want %q", tc.raw, resp.FinishReason, tc.want)
			}
		})
	}
}
