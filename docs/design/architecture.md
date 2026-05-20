# Architecture

Target: v0.1 (2026-07-06). See [v0.1-scope.md](v0.1-scope.md) for the In/Out scope. The diagrams below visualize decisions already made there — they don't introduce new ones.

## Component dependencies

The top-level `gateway` package is the **composition root**. No `pkg/*` knows about another `pkg/*`. `internal/testutil` is consumed only by `_test.go` files.

```mermaid
flowchart TB
    subgraph Top["gateway (composition root)"]
        GW["Config · New · Chat"]
    end

    subgraph Provider["pkg/provider"]
        IF["Provider interface"]
        OAI["openai"]
        ANT["anthropic"]
        IF --- OAI
        IF --- ANT
    end

    RT["pkg/router<br/>failover"]
    RL["pkg/ratelimit<br/>Redis sliding window"]
    MT["pkg/metrics<br/>Prometheus"]
    LG["pkg/logging<br/>slog"]
    TU["internal/testutil<br/>fixtures · mock Redis"]

    GW --> IF
    GW --> RT
    GW --> RL
    GW --> MT
    GW --> LG

    TU -. test only .-> IF
    TU -. test only .-> RT
    TU -. test only .-> RL
```

## Chat() request flow

Illustrative — current intent based on `v0.1-scope.md`. Exact wiring lands with ADR-002+.

```mermaid
sequenceDiagram
    autonumber
    participant App
    participant GW as gateway.Chat
    participant RL as pkg/ratelimit
    participant RT as pkg/router
    participant P as pkg/provider/*
    participant MT as pkg/metrics
    participant LG as pkg/logging

    App->>GW: Chat(ctx, ChatRequest)
    GW->>LG: log start (request_id)
    GW->>RL: Allow(model)?

    alt rate limited
        RL-->>GW: ErrLimited
        GW->>MT: record(rate_limited)
        GW-->>App: error
    else allowed
        RL-->>GW: ok
        GW->>RT: Route(primary, fallbacks)
        RT->>P: Chat(ctx, req)
        alt timeout / 5xx
            P-->>RT: error
            RT->>P: Chat(ctx, req) (next provider)
        end
        P-->>RT: response
        RT-->>GW: response + attempt trace
        GW->>MT: record(latency, tokens, cost, failover_count, reasons)
        GW->>LG: log done
        GW-->>App: response
    end
```

## Module boundary rules

| ✅ Allowed | ❌ Forbidden |
|---|---|
| `gateway` imports any `pkg/*` | `pkg/foo` imports `pkg/bar` |
| `pkg/*` imports `internal/testutil` (in `_test.go` only) | `pkg/*` imports `gateway` |
| `pkg/*` imports standard library + small, well-known third-party libs | Cyclic imports between any packages |
| Composition (wiring providers, router, ratelimit, metrics) lives in `gateway` | Composition leaking into `pkg/*` |

Enforcement: PR review for v0.1. Once we have working code, automate with `go vet -mod=mod` or a depgraph linter (separate ADR).
