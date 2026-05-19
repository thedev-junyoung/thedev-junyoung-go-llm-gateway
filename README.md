# go-llm-gateway

> Multi-provider LLM gateway for Go — failover, distributed rate limiting, observability in one library.

**Status:** Pre-release. Building v0.1 (target: 2026-07-06).

---

## Why

Python has [LiteLLM](https://github.com/BerriAI/litellm) (12k+ stars), [Portkey](https://github.com/Portkey-AI/gateway), and [Helicone](https://github.com/Helicone/helicone). The Go ecosystem has nothing comparable.

If you run LLM workloads from a Go service, you typically need:

- **Provider abstraction** — switch OpenAI ↔ Anthropic without rewriting call sites
- **Failover** — when one provider returns 5xx or times out, route to another
- **Distributed rate limits** — protect your API key budget across multiple service instances
- **Observability** — token usage, cost, latency per provider per model

Today, every Go team writes this from scratch. This project ships it as a library.

---

## Quick example (planned v0.1 API)

```go
gw, err := gateway.New(gateway.Config{
    Providers: []gateway.Provider{
        openai.New(os.Getenv("OPENAI_API_KEY")),
        anthropic.New(os.Getenv("ANTHROPIC_API_KEY")),
    },
    Routing: gateway.FailoverPolicy{
        Primary:   "openai",
        Fallbacks: []string{"anthropic"},
        Triggers:  gateway.OnTimeout | gateway.On5xx,
    },
    RateLimit: gateway.RedisRateLimit{
        Client:         redisClient,
        RequestsPerMin: 100,
        TokensPerMin:   50_000,
    },
    Metrics: gateway.PrometheusMetrics(prometheus.DefaultRegisterer),
})

resp, err := gw.Chat(ctx, gateway.ChatRequest{
    Model:    "gpt-4o", // automatically mapped to the active provider
    Messages: []gateway.Message{{Role: "user", Content: "Hello"}},
})
```

API is illustrative — final shape will land via ADR-002.

---

## v0.1 scope

- OpenAI + Anthropic providers
- Goroutine-based failover with timeout / 5xx triggers
- Redis-backed distributed rate limit (RPM + TPM, sliding window)
- Prometheus metrics + structured logging (`slog`)

Out of v0.1: semantic cache, streaming, embedding APIs, Gemini/Bedrock/Vertex, HTTP proxy mode. See [docs/design/v0.1-scope.md](docs/design/v0.1-scope.md).

---

## Documentation

- [Design — v0.1 scope](docs/design/v0.1-scope.md)
- [ADR-001 — Why a Go LLM gateway](docs/adr/0001-why-go-llm-gateway.md)
- [Roadmap](docs/roadmap.md)
- [Agent-driven development rules](docs/workflow/agent-driven-development.md)

---

## License

MIT (planned).
