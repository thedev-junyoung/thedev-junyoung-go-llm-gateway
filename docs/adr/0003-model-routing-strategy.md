# ADR-003 — Model→Provider Routing Strategy

- **Status:** Proposed *(agent-generated draft, pending maintainer review)*
- **Date:** 2026-05-29
- **Author:** Junyoung
- **비고:** 본 draft 의 결정/근거는 에이전트가 사용자 요청으로 작성. **각 Q 의 `Maintainer note` 줄**을 본인 voice 로 채우기 전엔 `Status` 를 `Accepted` 로 바꾸지 말 것.

---

## Context

ADR-002 가 `Provider` 인터페이스와 `SupportsModel(model string) bool` 메서드를 잠갔다. 그러나 `gateway.Chat(ctx, ChatRequest{Model: "gpt-4o"})` 가 호출됐을 때 **여러 등록된 provider 중 어디로 보낼지** 결정하는 알고리즘은 비어있다. router (Week 3) 구현 진입 전에 잠가야 코드 일관성이 유지된다.

본 ADR 의 결정은 다음 모듈에 영향:
- `pkg/router` — 본 ADR 의 알고리즘 구현체
- `gateway.Config.Providers` — slice 순서의 의미가 본 ADR 로 정의됨
- `pkg/provider/*` 어댑터 — `SupportsModel` 의 정확성이 router 정확성으로 직결

추가 제약:
- ADR-002 Q4 (`Model string` 그대로 노출, alias 없음)
- ADR-002 Q7 (`SupportsModel` 가 hot-path, network 호출 금지)
- v0.1 출시 일정 (~2026-07-06)

---

## Decision

### Q1. Routing 알고리즘

- **Decision:** A — **Config 순서대로 first-match by `SupportsModel`**.
- **Agent reasoning:** 명시 map (`map[string]ProviderName`) 은 v0.1 사용자에게 모든 모델을 사전 등록 강요 — 모델 셋이 빠르게 변하는 LLM 도메인에서 운영 부담. Provider 가 자기 모델 목록을 소유한다는 ADR-002 Q7 결정과 일관. 사용자는 `gateway.Config.Providers` slice 순서로 선호도 제어.
- **Maintainer note:** <!-- TODO: 본인 한 줄 voice 로 -->

### Q2. 한 모델을 여러 provider 가 지원할 때

- **Decision:** A — **Config 순서 first-match**.
- **Agent reasoning:** Q1 의 자연 귀결. 예: 사용자가 `Providers: [openai.New(...), azureOpenai.New(...)]` 로 등록하면 `"gpt-4o"` 는 OpenAI 로 (먼저 등록됨). 우선순위 바꾸려면 slice 순서 교체 — 명시적이고 추적 가능. `Routing.Primary` 같은 별도 필드 안 둠 (Q5 의 failover 와 의미 충돌 회피).
- **Maintainer note:** <!-- TODO: 본인 한 줄 voice 로 -->

### Q3. 모델 미지원 (어느 provider 도 `SupportsModel(m) == false`)

- **Decision:** A — **`Chat()` 즉시 `provider.NewProviderError("gateway", provider.ErrorTypeInvalidInput, 0, false, ...)` 반환** (`err.Vendor() == "gateway"`).
- **Agent reasoning:** 명시적 실패가 silent vendor reject 보다 디버깅 쉬움. Q3=B (primary 에게 떠넘김) 면 OpenAI 의 `"claude-opus-4-7"` 호출이 NotFound 로 reject 되어 사용자가 "왜 routing 실패가 아니라 vendor 404 지?" 혼란. 새 sentinel (`ErrModelUnsupported`) 도입 (Q3=C) 까지는 안 함 — 기존 `ErrorTypeInvalidInput` 카테고리가 충분히 의미 전달. error 의 `Vendor()` 는 `"gateway"` (origin 이 router 임을 표시).
- **Maintainer note:** <!-- TODO: 본인 한 줄 voice 로 -->

### Q4. Alias / 가상 모델명 (`"fast"`, `"cheap"`, `"default"`)

- **Decision:** A — **v0.1 미지원, Open Question 으로 보존**.
- **Agent reasoning:** YAGNI. 사용 사례 (`"fast"` 가 어느 provider 의 어느 모델로 가는지) 가 사용자/팀마다 다름 — gateway 가 추정하면 silent magic. v0.2 ADR 에서 실제 요청 등장 시 결정. v0.1 사용자가 정말 필요하면 application layer 에서 string 변환해서 ChatRequest.Model 에 박으면 됨.
- **Maintainer note:** <!-- TODO: 본인 한 줄 voice 로 -->

### Q5. Failover 와의 상호작용

- **Decision:** B — **Failover 는 같은 모델을 지원하는 fallback 만 시도**.
- **Agent reasoning:** Failover 가 모델 자동 translate (예: `gpt-4o` → `claude-opus-4-7`) 하면 사용자가 모르는 사이 응답 특성 바뀜 — temperature/format/cost/지능 다른 모델로 silent 변환. ADR-002 "uniform interface" 약속 위배. 안전한 룰: router 가 fallback 목록을 순회하며 `SupportsModel(req.Model)` true 인 provider 만 시도. 명시 mapping (`ModelFallbacks map[string][]string`, Q5=C) 은 알고리즘 복잡도 ↑ + 또 다른 config 표 — v0.1 에는 과함. 필요 사례 등장 시 v0.2 ADR.
- **Maintainer note:** <!-- TODO: 본인 한 줄 voice 로 -->

---

### Synthesis — Go 시그니처

```go
package router

import (
	"fmt"

	"github.com/thedev-junyoung/thedev-junyoung-go-llm-gateway/pkg/provider"
)

// Pick returns the first provider in `providers` whose SupportsModel(model)
// returns true. The argument order IS the priority — gateway.Config.Providers
// is passed through verbatim.
//
// Returns a *provider.ProviderError with Type == ErrorTypeInvalidInput and
// Vendor() == "gateway" when no provider supports the model. The router origin
// is encoded in the vendor field so call sites can distinguish a routing
// failure from a vendor-side rejection.
func Pick(providers []provider.Provider, model string) (provider.Provider, error) {
	for _, p := range providers {
		if p.SupportsModel(model) {
			return p, nil
		}
	}
	return nil, provider.NewProviderError(
		"gateway",
		provider.ErrorTypeInvalidInput,
		0,
		false,
		fmt.Sprintf("no registered provider supports model %q", model),
		nil,
	)
}

// PickWithFallbacks returns the primary and the fallback chain of providers
// that support `model`, in config order. Used by the failover loop:
//
//   primary, fallbacks, err := router.PickWithFallbacks(cfg.Providers, req.Model)
//   ... try primary, on retriable failure try fallbacks in order ...
//
// Providers that do NOT support the model are excluded from both lists —
// per ADR-003 Q5, the gateway never auto-translates models across providers.
func PickWithFallbacks(providers []provider.Provider, model string) (primary provider.Provider, fallbacks []provider.Provider, err error) {
	supporting := make([]provider.Provider, 0, len(providers))
	for _, p := range providers {
		if p.SupportsModel(model) {
			supporting = append(supporting, p)
		}
	}
	if len(supporting) == 0 {
		return nil, nil, provider.NewProviderError(
			"gateway", provider.ErrorTypeInvalidInput, 0, false,
			fmt.Sprintf("no registered provider supports model %q", model), nil,
		)
	}
	return supporting[0], supporting[1:], nil
}
```

> **Note for maintainer:** README "Quick example" 의 `Routing: gateway.FailoverPolicy{...}` 모양과 위 시그니처가 일관되는지 본인 review 시 확인. `FailoverPolicy.Primary` 같은 명시 필드를 README 가 보여주고 있는데, 본 ADR 은 그 필드를 도입하지 않는다 (Config.Providers 순서가 우선권) — README 와 일치시키는 follow-up PR 필요.

---

## Alternatives Considered

### Alt 1 — Q1 에서 명시 map (옵션 B: `map[string]ProviderName`)

- **장점:** 모델→provider 매핑이 declarative 하게 한 곳에 보임. 새 모델 추가 시 코드 변경 한 군데.
- **단점:** Provider 가 자기 모델 목록을 이미 알고 있는데 (ADR-002 Q7), 사용자가 그것을 또 적어야 하는 중복. 새 모델 출시 → adapter 의 default models 와 사용자 map 둘 다 갱신해야 — drift 위험. Map 비어있으면 라이브러리가 routing 못 함 (반면 first-match 는 SupportsModel 만 정확하면 zero-config 동작).
- **안 선택한 이유:** Config 부담 vs 사용자 통제 트레이드오프. ADR-002 가 이미 provider 에 모델 소유권을 줬으므로 같은 결정의 연장이 일관성. Alias 필요한 사용자는 Q4 의 v0.2 ADR 또는 application layer 변환으로 해결.

### Alt 2 — Q5 에서 모델 자동 translate (옵션 A)

- **장점:** "OpenAI 다운 → Anthropic 자동 시도" 같은 가용성 극대화. 사용자 친화적으로 보임.
- **단점:** `gpt-4o` 와 `claude-opus-4-7` 의 응답 톤·temperature 해석·max_tokens 의미·tool calling 지원 다름. 사용자가 모르는 사이 응답 특성 변경 → 예측 가능성 ↓. cost 도 다름 (관측 가능성 시점에 사용자 confused). ADR-002 Consequences/Negative 의 "uniform interface 부분 충돌" 항목이 정확히 이 시나리오의 경고.
- **안 선택한 이유:** v0.1 의 정직성 우선. 가용성 vs 예측 가능성에서 후자. 자동 translate 가 정말 필요한 사용 사례 (예: 코드 자동완성 - 어느 모델이든 OK) 는 application layer 에서 ChatRequest.Model 을 명시적으로 바꿔 두 번 호출하면 같은 효과 + 호출자가 인지.

### Alt 3 — Q3 에서 새 sentinel `ErrModelUnsupported` (옵션 C)

- **장점:** Router-specific 실패와 vendor-side InvalidInput 을 sentinel 로 명확히 구별. `errors.Is(err, router.ErrModelUnsupported)` 가능.
- **단점:** Sentinel 추가는 router-critical 4개 sentinel 룰 (ADR-002 Q5) 위배 — model unsupported 는 retry/failover/abort 결정에 직접 안 쓰임 (항상 abort). ErrorType + Vendor 조합 (`Type==ErrorTypeInvalidInput && Vendor()=="gateway"`) 으로 충분히 구별 가능.
- **안 선택한 이유:** Sentinel 인플레이션 회피. v0.2 에서 router 가 분기에 직접 쓰는 새 시그널 등장 시 그때 sentinel 승격.

---

## Consequences

### Positive

- Zero-config 운영 — 사용자가 `gateway.Config.Providers` 에 어댑터만 등록하면 routing 자동 동작
- Provider 가 자기 모델 셋 소유 → 새 모델 추가가 어댑터 PR 한 군데로 끝 (사용자 config 변경 불필요)
- Failover 가 같은 모델 지원 provider 만 시도 → "왜 응답이 갑자기 달라졌지?" 류 silent magic 없음
- Routing 실패의 vendor 가 `"gateway"` 로 표시 → log 만 봐도 router vs vendor 실패 즉시 구별

### Negative

- 사용자가 매핑을 한 눈에 못 봄 — 모델→provider 가 declarative 가 아니라 `SupportsModel` 호출 결과의 합성. `gateway.DebugRouting(providers, model)` 같은 helper 가 향후 필요할 수 있음 (v0.2 검토)
- 같은 모델을 OpenAI + Azure 둘 다 지원하면 Config 순서로만 우선권 — 더 정교한 룰 (지역별 routing, cost-based) 은 v0.2+
- 모델 미지원 케이스가 vendor 호출까지 안 가므로 모델 오타가 "이 provider 가 deprecate 한 건지" vs "내가 잘못 적은 건지" 구별 안 됨 — error message 에서 등록된 provider 목록을 노출하는 것은 follow-up

### Risks

| Risk | Mitigation |
|---|---|
| 어댑터의 `SupportsModel` 가 stale (모델 deprecate 됨) → router 가 stale provider 로 보내고 vendor 가 NotFound | Dependabot 으로 어댑터 업데이트 추적 + 어댑터 PR 의 model list 정합성 review |
| 사용자가 `Providers` slice 순서를 부주의하게 변경 → 동일 모델이 다른 provider 로 routing → 응답 톤 변경 | doc 강조: slice 순서가 의미를 가짐. `gateway.DebugRouting` helper (v0.2) 로 가시화 |
| Q5 결정으로 가용성 손실 (primary 다운 + fallback 이 그 모델 미지원 시 전체 실패) | sentinel `ErrRateLimited`/`ErrOverloaded`/`ErrServer` 가 retriable — router 가 같은 provider 에 backoff 후 재시도. 진짜 가용성 필요한 경우 사용자가 같은 모델 지원하는 provider 를 명시적으로 등록 |

---

## Related ADRs

- [ADR-001](0001-why-go-llm-gateway.md) — failover 가 본 라이브러리의 핵심 가치 (본 ADR Q5 가 그 가치의 명시적 안전장치)
- [ADR-002](0002-provider-interface-design.md) — Q4 (`Model string`) + Q7 (`SupportsModel`) 가 본 ADR 의 직접 입력
- **ADR-004 (예정)** — Failover trigger 조건 + 재시도 정책. 본 ADR 은 *어떤 provider 가 후보인지*, ADR-004 는 *언제 다음 후보로 넘어가는지*

---

## Open Questions

- [ ] Alias / 가상 모델명 (`"fast"`, `"cheap"`, `"default"`) — v0.2 ADR. 실제 사용 사례 (issue / 사용자 피드백) 등장 시 다시.
- [ ] Cost-based routing — 가장 싼 provider 에 먼저 routing? 응답 품질 vs 비용 트레이드오프. v0.2+ 검토.
- [ ] Region-based routing — 사용자가 Azure 의 특정 region 등 명시 — `Provider` 인터페이스 확장 필요 (`Region() string`?).
- [ ] `gateway.DebugRouting(providers, model) (chosen, fallbacks, err)` helper — 사용자가 routing 결과를 코드로 자체 검증 가능하게.
- [ ] `Chat()` 시 routing 실패 error message 에 등록 provider 목록 + 각 provider 의 지원 모델 노출 — 디버깅 친화성.
