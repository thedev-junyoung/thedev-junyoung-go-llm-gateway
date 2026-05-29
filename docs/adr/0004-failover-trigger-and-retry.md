# ADR-004 — Failover Trigger Conditions + Retry Policy

- **Status:** Proposed *(agent-generated draft, pending maintainer review)*
- **Date:** 2026-05-29
- **Author:** Junyoung
- **비고:** 본 draft 의 결정/근거는 에이전트가 사용자 요청으로 작성. 각 Q 의 `Maintainer note` 줄을 본인 voice 로 채우기 전엔 `Status` 를 `Accepted` 로 바꾸지 말 것.

---

## Context

- ADR-002 가 `*provider.ProviderError` 에 `Type` / `Retriable` / `RetryAfter` 필드를 노출
- ADR-003 의 `router.PickWithFallbacks` 가 primary + fallback chain 을 반환
- 현재 `gateway.Chat` (PR #55) 은 primary 만 호출 — failover 없음
- 본 ADR 은 *어떤 에러에서 fallback 으로 넘어가는지*, *얼마나 빨리*, *언제 포기하는지* 를 결정

영향 모듈:
- `gateway.Chat` (orchestration loop)
- `pkg/router` (failover trace 정보 노출 — Week 5 metrics 입력)
- `pkg/metrics` (Week 5; 본 ADR 의 trace 형식이 입력 명세)

추가 제약:
- ADR-001 의 "production-grade Go gateway" 정체성 → failover 가 본 라이브러리의 핵심 가치
- ADR-002 의 sentinel 4개 (`ErrRateLimited` / `ErrOverloaded` / `ErrAuthFailed` / `ErrTimeout`) + `Retriable bool` 가 이미 분류 도구
- v0.1 출시 6주 일정 — 복잡한 backoff/circuit-breaker 같은 거 도입 금지

---

## Decision

### Q1. Failover 를 trigger 하는 에러

- **Decision:** B — **모든 `*ProviderError` 중 `Retriable == true` 인 것**.
- **Agent reasoning:** 분류 로직이 이미 어댑터에 한 번 박혀 있음 (ADR-002 + OpenAI/Anthropic 어댑터의 `mapHTTPError`). router 가 또 다른 trigger 룰 (예: "rate_limit 만 / server 는 빼고") 을 세우면 두 곳에 분기 룰이 생기고, 새 ErrorType 추가 시 둘 다 갱신해야 — drift 위험. `Retriable` bool 은 어댑터가 *"이 에러는 다음에 다시 시도할 가치가 있다"* 를 한 점에서 단언하는 sentinel — router 는 그 단언을 신뢰.
- **왜 이 결정이 정당한가:** 어댑터가 가장 vendor 를 잘 안다. router 가 trigger 룰을 별도로 가지면 vendor 변경 (예: Anthropic 이 새 error type 추가) 시 router 코드가 영향받음 — 그건 layering 위반.
- **Maintainer note:** <!-- TODO: 본인 한 줄 voice 로 -->

### Q2. 같은 provider 재시도 (in-provider retry before failover)

- **Decision:** B — **재시도 없음. 첫 retriable 에러에 즉시 다음 provider 시도**.
- **Agent reasoning:** v0.1 의 핵심 가치 = *failover* (다른 provider 로 빨리 전환). In-provider retry 는 의미가 다른 기능:
  - rate_limit/overloaded 같은 케이스에서 같은 vendor 가 곧 회복될 수도 있지만, 그 회복 가능성보다 **이미 등록된 다른 vendor 가 즉시 응답 가능** 할 확률이 (보통) 더 높음
  - 같은 provider retry 면 `Retry-After` 대기 동안 caller 가 blocking — 가용성 손실
  - In-provider retry 도입은 retry budget / backoff / jitter / circuit-breaker 같은 후속 결정들이 줄줄이 따라옴 → v0.1 scope 부풀림
- **왜 이 결정이 정당한가:** "fail-fast + lateral switch" 가 multi-provider gateway 의 의도. 같은 vendor 에 매달리는 retry 는 single-provider deployment 의 패턴 — 본 라이브러리 존재 의미를 약화.
- **Maintainer note:** <!-- TODO: 본인 한 줄 voice 로 -->

### Q3. Failover 시도 간 backoff

- **Decision:** A — **즉시 다음 provider 로 (sleep 없음)**.
- **Agent reasoning:** `RetryAfter` 는 *"같은 vendor 에 재시도하기 전에 기다려라"* 는 의미. *다음 vendor* 호출에는 적용 의미가 없음 (Anthropic 의 Retry-After 가 OpenAI 의 적정 호출 간격을 알려주는 건 아님). 본 라이브러리의 sentinel `RetryAfter` 필드는 v0.1 시점에는 *호출자의 정보 제공용* 만 — router 자체는 즉시 다음으로 이동.
- **왜 이 결정이 정당한가:** failover 의 wall-clock latency 가 단일 vendor 호출의 1.x 배를 넘기면 사용자 경험 손상. backoff 추가는 *retry* 의미에서만 정당한데, Q2 가 retry 없음으로 결정 → backoff 도 자연 소멸.
- **Maintainer note:** <!-- TODO: 본인 한 줄 voice 로 -->

### Q4. 전체 request 시간 예산

- **Decision:** A — **caller 의 `context.Context` deadline/cancel 이 진실의 원천**.
- **Agent reasoning:** Gateway 가 자체 default timeout 을 들고 있으면 caller 의 `WithTimeout(ctx, 90s)` 와 충돌 (어느 게 이기나? 사용자 추정 어려움). caller 가 명시 안 하면 *그게 caller 의 명시적 결정* 이고, 그땐 어댑터의 `http.Client.Timeout` (현재 60s) 이 last-line. gateway 가 또 layer 추가하면 timeout 계층이 셋 (gateway / client / vendor) — 디버깅 악몽.
  - 매 attempt 사이 `ctx.Err()` 체크 → caller 가 cancel 하면 다음 시도 안 함
  - `context.DeadlineExceeded` 가 어댑터에서 올라오면 `ErrTimeout` (Retriable=true) 으로 failover 시도하지만, 만약 ctx 가 진짜 만료된 상태면 다음 attempt 의 첫 줄에서 즉시 abort
- **왜 이 결정이 정당한가:** Go 관용. caller 가 자기 timeout 을 알고 라이브러리는 그걸 존중. "라이브러리가 알아서 30초 잘라줌" 류는 Python 적 사고 — Go 에선 context 가 표준.
- **Maintainer note:** <!-- TODO: 본인 한 줄 voice 로 -->

### Q5. Attempt trace 노출

- **Decision:** **본 ADR 에서 미결정** — ADR-006 (observability) 으로 미룸.
- **Agent reasoning:** v0.1 floor (이번 PR) 는 *내부* failover loop 만 작동시키고, attempt 정보 노출 형식 (struct? context value? metric only?) 은 Week 5 metrics ADR 의 입력. 지금 미리 정하면 metric 요구사항을 추정해서 그리는 셈 — 거꾸로.
  - 내부 구현: failover loop 은 매 attempt 의 `(provider.Name, err, duration)` 을 slice 에 모음 (gateway 내부 변수). v0.1 에서는 `gateway.Chat` 반환값에 안 포함, 로깅도 안 함.
  - Week 5 에서 metric collector 가 이 slice 를 어떻게 받을지 결정 (예: ChatResponse 에 필드 추가 / context callback / interface 주입).
- **왜 이 결정이 정당한가:** premature design 방지. metric 모듈이 실제 요구 사항을 표현하는 시점에 trace 형식 결정. 본 ADR 은 "loop 작동" 까지만.
- **Maintainer note:** <!-- TODO: 본인 한 줄 voice 로 (또는 "ADR-006 으로 미룬 결정 OK") -->

---

### Synthesis — Go pseudocode

```go
package gateway

import (
	"context"
	"errors"
	"fmt"

	"github.com/thedev-junyoung/thedev-junyoung-go-llm-gateway/pkg/provider"
	"github.com/thedev-junyoung/thedev-junyoung-go-llm-gateway/pkg/router"
)

func (g *Gateway) Chat(ctx context.Context, req provider.ChatRequest) (provider.ChatResponse, error) {
	primary, fallbacks, err := router.PickWithFallbacks(g.providers, req.Model)
	if err != nil {
		return provider.ChatResponse{}, err // routing failure: no eligible provider
	}

	// Single ordered chain — primary first, then fallbacks in config order.
	candidates := append([]provider.Provider{primary}, fallbacks...)

	var lastErr error
	for i, p := range candidates {
		// Caller's context wins — abort before next attempt if cancelled.
		// Dual-wrap (Go 1.20+) preserves BOTH the ctx error and the most
		// recent vendor error so callers can errors.Is for either:
		//   errors.Is(err, context.Canceled)         // detect cancellation
		//   errors.Is(err, provider.ErrRateLimited)  // why we stopped
		if cerr := ctx.Err(); cerr != nil {
			if lastErr != nil {
				return provider.ChatResponse{}, fmt.Errorf("%w: last vendor error: %w", cerr, lastErr)
			}
			return provider.ChatResponse{}, cerr
		}

		resp, err := p.Chat(ctx, req)
		if err == nil {
			return resp, nil // success — done
		}
		lastErr = err

		if !shouldFailover(err) {
			return provider.ChatResponse{}, err // non-retriable: abort, do not try fallbacks
		}
		// retriable: continue to next candidate.
		_ = i // (later: record attempt trace per ADR-006)
	}

	// Exhausted all candidates with retriable failures.
	return provider.ChatResponse{}, lastErr
}

// shouldFailover returns true iff err is a *provider.ProviderError marked
// Retriable. Unknown error types (e.g. a bug surfacing a raw error) do NOT
// trigger failover — surfaced as-is so the next vendor isn't blamed for our
// own defect.
func shouldFailover(err error) bool {
	var pe *provider.ProviderError
	if !errors.As(err, &pe) {
		return false
	}
	return pe.Retriable
}
```

> **Note for maintainer:** 위 `_ = i` 는 ADR-006 에서 attempt trace 수집 코드가 들어갈 placeholder. v0.1 구현 PR 에서는 그 자리에 주석으로 표시.

---

## Alternatives Considered

### Alt 1 — Q2 에서 in-provider retry (옵션 A)

- **장점:** rate-limit/overloaded 같이 곧 회복 가능한 에러에서 가용성 ↑. 단일 vendor 만 등록한 사용자에게도 의미 있음.
- **단점:** Retry budget / backoff curve / jitter / circuit-breaker 같은 줄줄이 따라오는 결정들이 v0.1 scope 폭증. caller 가 wait 동안 blocking — failover 의 "빠른 전환" 가치 약화.
- **안 선택한 이유:** *Multi-provider* gateway 의 의도. 단일 vendor retry 가 정말 필요한 사용자는 application layer 또는 vendor SDK 의 retry 기능 이용. v0.2 에서 *Smart retry policy* 별도 ADR 로 검토.

### Alt 2 — Q3 에서 Retry-After 대기 (옵션 B)

- **장점:** vendor 의 정확한 hint 존중 → 다음 attempt 의 즉시 실패 가능성 ↓.
- **단점:** Q3 가 Q2 의 "retry" 의미일 때만 정당. failover 의 "다른 vendor 호출" 에는 적용 의미가 없음 — Anthropic Retry-After 가 OpenAI 와 무관.
- **안 선택한 이유:** semantic mismatch. v0.1 에서 RetryAfter 는 caller 가 ProviderError 검사 시 활용 (호출자가 자체 retry 결정).

### Alt 3 — Q4 에서 wall-clock 예산 (옵션 B)

- **장점:** 각 vendor 가 30초씩 hang 하면 전체 5 vendor × 30s = 2.5분 — 사용자 한도 보호.
- **단점:** caller 의 `WithTimeout` 와 layering 충돌. 어느 게 이길지 사용자가 추정 어려움. Go context 표준 우회.
- **안 선택한 이유:** Go 관용 (context 우선). caller 가 한도를 알고 명시. 본 라이브러리가 "30초 default" 같은 magic 도입하면 사용자가 그걸 발견하기까지 디버깅 시간 낭비.

---

## Consequences

### Positive

- 어댑터의 `Retriable bool` 단언 한 곳이 trigger 룰 진실의 원천 → router 코드가 vendor 변경에 영향 안 받음
- "fail-fast + lateral switch" 의미가 명료 — failover 의 wall-clock cost 가 단일 attempt 의 N배가 아니라 거의 1배
- caller 의 context 가 진실의 원천 → timeout 계층 충돌 없음. Go 관용
- 비-retriable 에러 (Auth/Permission/InvalidInput) 가 즉시 abort → 잘못된 input 으로 모든 provider 호출하는 낭비 방지

### Negative

- 같은 vendor 가 곧 회복 가능한 케이스 (RateLimit + 짧은 RetryAfter) 도 즉시 다음 vendor 로 → 다음 vendor 가 더 비싸거나 품질 낮으면 사용자 경험 미묘하게 변경. 사용자가 인지 못 하면 cost spike 가능
- Attempt trace 가 v0.1 에서 외부 노출 안 됨 → 디버깅 시 어떤 vendor 가 어떤 에러로 실패했는지 caller 가 즉시 모름 (로그 직접 봐야). ADR-006 까지 임시 결함
- ctx-only 예산이라 caller 가 timeout 안 주면 전체 chain 이 어댑터 default (60s) × N 만큼 hang 가능. 운영 시 doc 강조 필요

### Risks

| Risk | Mitigation |
|---|---|
| 모든 provider 가 같은 에러 (예: invalid model) → loop 끝까지 돌고 같은 에러 반환 | router.PickWithFallbacks 가 이미 SupportsModel 로 필터 → 모델 매핑 실패는 routing 시점에 InvalidInput 으로 즉시 abort (ADR-003) |
| ctx 가 없거나 background → 무한정 hang | 어댑터의 `http.Client.Timeout` (60s) 가 last-line. doc 에 "always pass ctx with WithTimeout" 명시 권장 |
| Retriable 분류 오류 (어댑터가 잘못 분류) → 영원히 failover 되거나 영원히 안 되거나 | 어댑터 단위 테스트가 status code → ErrorType 매트릭스 검증 중. 새 어댑터 추가 시 같은 test 패턴 강제 |
| 같은 vendor 다중 등록 시 (예: openai + Azure OpenAI) 첫 번째 다운이면 두 번째도 같은 confluent 이슈로 동시 다운 가능 | 사용자가 다른 region/account 의 같은 vendor 와 다른 vendor 를 적절히 섞도록 doc + example 가이드 |

---

## Related ADRs

- [ADR-001](0001-why-go-llm-gateway.md) — failover 가 본 라이브러리의 핵심 가치
- [ADR-002](0002-provider-interface-design.md) — Q5 의 `Retriable` / `RetryAfter` 가 본 ADR Q1/Q3 의 직접 입력
- [ADR-003](0003-model-routing-strategy.md) — `PickWithFallbacks` 가 본 ADR 의 candidate chain 공급원
- **ADR-005 (예정)** — Distributed rate limit. 본 ADR 의 "no in-provider retry" 결정과 rate limit 의 retry 정책이 합쳐질 가능성
- **ADR-006 (예정)** — Observability. 본 ADR Q5 의 attempt trace 노출 형식 결정

---

## Open Questions

- [ ] Attempt trace 외부 노출 형식 — ADR-006 입력
- [ ] In-provider retry / circuit-breaker — v0.2 *Smart retry policy* 별도 ADR (실제 사용자 요구 등장 시)
- [ ] 같은 vendor 다중 등록 시 confluent failure 감지 (Q-risk 항목 중 4번) — observability 가 land 한 후 데이터 기반 결정
- [ ] Provider 단위 default timeout 의 사용자 override API — `provider.New` 의 `WithHTTPClient` 만으로 충분한지 검토
- [ ] Failover decision 이 trace 에 남는 형식 (ErrorType + StatusCode + duration 만? 아니면 ProviderError 전체 직렬화?) — ADR-006 입력
