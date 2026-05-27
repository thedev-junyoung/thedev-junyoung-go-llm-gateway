# ADR-002 — Provider Interface Design

- **Status:** Proposed *(agent-generated draft, pending maintainer review)*
- **Date:** 2026-05-21
- **Author:** Junyoung
- **비고:** 본 draft 의 결정/근거는 에이전트가 사용자 요청으로 작성했음. **각 Q 의 `Maintainer note` 줄**을 본인 voice 로 한 줄씩 채우기 전에는 `Status` 를 `Accepted` 로 바꾸지 말 것.

---

## Context

`go-llm-gateway` v0.1 은 OpenAI 와 Anthropic 두 벤더를 지원한다. 두 벤더의 요청/응답/에러 모양이 다르다. 본 ADR 결정에 영향을 준 핵심 차이를 ADR 안에 인라인으로 못 박는다 (검증일: 2026-05-21):

| 항목 | OpenAI | Anthropic |
|---|---|---|
| `system` 메시지 | `messages` 배열 내 `{role:"system"}` (복수 가능) | top-level `system` 필드 (단일 string/array) |
| `max_tokens` | optional | **required** |
| 응답 `content` | string (`choices[0].message.content`) | **array of blocks** (`[{type:"text",text:...}, {type:"tool_use",...}]`) |
| stop/finish reason | `stop` / `length` / `content_filter` / `tool_calls` | `end_turn` / `max_tokens` / `stop_sequence` / `tool_use` |
| usage 필드 | `prompt_tokens` / `completion_tokens` / `total_tokens` | `input_tokens` / `output_tokens` (+ optional `cache_*`) |
| error type 예 | `invalid_request_error`, `rate_limit_exceeded`, `server_error` | `invalid_request_error`, `rate_limit_error`, `api_error`, `overloaded_error` (Anthropic 특유) |

(작업용 cheat sheet 는 로컬 `scratch/provider-cheat-sheet.md` — repo 추적 안 됨.)

다음 모듈들이 의존할 통일된 `Provider` 추상화가 필요하다:
- `pkg/router` — failover 시 다른 provider 로 swap
- `pkg/ratelimit` — provider 별 RPM/TPM 관리
- `pkg/metrics` — provider/모델별 latency, cost, token 사용량 기록

이 인터페이스는 v0.1.0 으로 출시되면 모든 사용자 코드가 의존하므로 **변경 비용이 크다.** 따라서 ADR 로 결정을 잠근다.

추가 제약:
- Go 관용 스타일 (`context.Context` 첫 인자, `error` 반환, exported doc 주석 최소)
- v0.1 출시 6주 일정 (~2026-07-06). breaking change 후속 ADR 가능하지만 v0.1.0 → v0.2 에는 한 번 정도만.

---

## Decision

### Q1. 메서드 셋

- **Decision:** A — `Chat(ctx, req) (resp, error)` **하나만**.
- **Agent reasoning:** YAGNI. `Stream()` / `Embed()` 메서드는 v0.1 out of scope. `ErrNotImplemented` stub 은 인터페이스 부풀리기만 하고 실제 가치 없음. v0.2 에서 `Stream()` 추가는 breaking change 1회, 그건 별도 ADR. **참고:** `Chat()` 의 **결과 안에서** `FinishToolUse` / tool_use content block 이 나타나는 것은 v0.1 에서도 가능 (벤더가 응답 끝나면서 "tool 호출 필요" 시그널 보내는 케이스). Stream/Embed 와 tool_use 신호 노출은 별개 — tool_use 신호는 그저 응답 상태일 뿐 별도 메서드 필요 없음.
- **Maintainer note:** <!-- TODO: 본인 한 줄 voice 로 -->

### Q2. Request — `system` 모델링

- **Decision:** B — `System string` 필드 분리 + `Messages []Message`.
- **Agent reasoning:** Anthropic 은 `system` 이 top-level 필수 구조 (cheat sheet 1번). OpenAI 는 어느 쪽이든 변환 가능. 더 엄격한 쪽(Anthropic) 모양을 외부 API 로 노출하면 OpenAI 어댑터는 trivial 하게 messages 배열로 변환 가능 — 반대 방향은 lossy (OpenAI 의 multiple system messages 를 Anthropic 의 단일 system 으로 어떻게?).
- **Maintainer note:** <!-- TODO: 본인 한 줄 voice 로 -->

### Q3. Response 구조

- **Decision:** B — `Content string` + `FinishReason` + `Usage` + `Raw json.RawMessage`.
- **Agent reasoning:** Content 를 string 으로 통일하려면 Anthropic blocks 를 text-block 만 추출해서 join 해야 함. tool_use 만 있는 응답 등 edge case 는 join 결과가 빈 문자열 → `Raw` 가 fallback (사용자가 직접 파싱).
  CostUSD 는 Provider 책임 아님 (단가표는 `pkg/metrics` 소유). Provider 는 thin 하게.
  **알려진 한계:** image / document / tool_use block 같은 multi-modal content 는 `Content string` 만으로 표현 불가 → 사용자는 `Raw` 로 fallback. 텍스트 + 멀티모달 혼합 케이스가 v0.2 에서 흔해지면 인터페이스 진화 필요 (Open Question 참고).
- **Maintainer note:** <!-- TODO: 본인 한 줄 voice 로 -->

### Q4. 모델 식별

- **Decision:** A — `Model string` (벤더 모델명 그대로: `gpt-4o`, `claude-opus-4-7`).
- **Agent reasoning:** `ModelID` 새 타입은 v0.1 시점에 공유 모델 지식이 없어서 가치 < friction. Alias (`"fast"` → 활성 provider 의 fast 모델) 는 abstraction over abstraction — 어느 alias 가 어디로 가는지 user 가 추적 못 함. Router 의 model→provider 매핑은 ADR-003 에서 별도로.
- **Maintainer note:** <!-- TODO: 본인 한 줄 voice 로 -->

### Q5. 에러 타입 (가장 중요)

- **Decision:** C — `*ProviderError{Type, Retriable, RetryAfter, StatusCode, Message, vendor (unexported), Wrapped}` + sentinel errors (`ErrRateLimited`, `ErrOverloaded`, `ErrAuthFailed`, `ErrTimeout` — **router-critical 4개**).
- **Agent reasoning:** Router 가 retry / failover / abort 를 정확히 판단하려면 (1) 분류된 type (`Retriable bool` 포함), (2) router 분기에 직접 쓰이는 카테고리는 sentinel 로 `errors.Is(err, ErrRateLimited)` ergonomics. 단일 `error` (옵션 A) 면 call site 마다 string match 발생 — anti-Go. typed only (옵션 B) 면 sentinel ergonomics 없음. Anthropic 의 `overloaded_error` 같은 특유 시그널을 통일 enum 으로 카테고라이즈하는 가치가 있음.
  **Sentinel 4개로 한정한 이유:** rate_limit / overloaded / auth / timeout 은 router 가 분기 (retry vs failover vs abort) 에 직접 사용. 나머지 5개 카테고리 (`Permission`, `InvalidInput`, `NotFound`, `Server`, `Unknown`) 는 분류만 필요하고 분기 안 함 → 사용자는 `var pe *ProviderError; errors.As(err, &pe); pe.Type` 패턴으로 접근. sentinel 추가 시점은 실제 사용 사례 등장 시.
  **`RetryAfter *time.Duration`:** 벤더가 `Retry-After` 헤더로 보내는 backoff 힌트를 router 에 그대로 전달. Router 가 backoff duration 을 자체 추정하면 vendor 가 알려준 정확한 대기 시간을 버리는 손실 발생.
  **`vendor` unexported + `Vendor()` accessor:** Negative 항목에서 경고한 `if err.Vendor == "openai"` 안티패턴을 컴파일 타임에 차단. 로깅/디버깅 read 는 accessor 로.
  **Constructor — `NewProviderError(vendor, type, statusCode, retriable, msg, wrapped)`:** `vendor` 가 unexported 이므로 `pkg/provider/openai`, `pkg/provider/anthropic` 같은 별도 어댑터 패키지가 struct literal 로 초기화 불가. 이 constructor 가 어댑터 패키지에서 `vendor` 를 set 하는 유일한 진입점. `RetryAfter *time.Duration` 은 vendor 응답에 헤더가 있을 때만 채워지는 optional 필드 → 별도 chainable setter `WithRetryAfter(d time.Duration) *ProviderError` 로 분리 (생성자 인자 폭증 회피).
- **Maintainer note:** <!-- TODO: 본인 한 줄 voice 로 -->

### Q6. Context propagation

- **Decision:** A — `context.Value` 로 request_id 흐름. **Helper 와 key 는 `internal/requestctx`** 에 위치, `gateway` 패키지(composition root)가 외부용 wrapper 노출.
- **Agent reasoning:** Go 관용. 외부 API 가 `RequestID string` 필드를 강제하면 사용자 코드가 매번 UUID 생성해야 → 부담. context 기반은 미들웨어가 주입하는 표준 패턴. Key 를 string 으로 쓰면 다른 패키지와 충돌 가능 — `type contextKey struct{ name string }` 패턴 (단일 unexported 타입에 `name` 필드, 패키지별 sentinel 변수) 사용.
  **위치 결정 (`internal/requestctx`):** `docs/design/architecture.md` 의 모듈 경계 규칙 — *"어떤 `pkg/*` 도 다른 `pkg/*` 를 모른다"* — 을 지키려면 cross-cutting helper 가 `pkg/*` 안에 있을 수 없다. helper 가 `pkg/provider` 에 있으면 `pkg/logging`, `pkg/router`, `pkg/metrics` 가 request id 를 읽기 위해 `pkg/provider` 를 import 해야 하고 이는 layering 정면 위반. `internal/` 패키지는 `pkg/*` 가 아니라 외부 공개되지 않는 공유 경계 → 모든 `pkg/*` 가 자유롭게 import 가능. 외부 사용자는 `gateway.WithRequestID(...)` 한 곳을 호출 (composition root 에서 `internal/requestctx` 위로 thin wrapping). 옵션 C (`pkg/requestid` 외부 노출 패키지) 도 같은 layering 위반을 그대로 반복하므로 기각.
- **Maintainer note:** <!-- TODO: 본인 한 줄 voice 로 -->

### Q7. Provider 메타 메서드

- **Decision:** B — `Name() string` + `SupportsModel(model string) bool`.
- **Agent reasoning:** Router 가 "gpt-4o 호출인데 어느 provider 가능한가?" 판단할 때, 모델 지식이 provider 구현 옆에 있는 게 응집도 ↑. Config-only mapping (옵션 C) 은 사용자가 매번 매핑 표 유지 — fragile.
  **솔직한 트레이드오프:** `SupportsModel` 도 모델 목록을 코드에 박는 한 fragile 함은 동일 (벤더가 모델 deprecate / 신규 출시 / 이름 변경 시마다 PR 필요). 차이는 "유지보수 책임이 라이브러리 안 vs 사용자 config" 일 뿐. 라이브러리 측이 짊어지는 게 (1) 사용자 코드에서 모델명 hard-code 안 해도 됨, (2) 신규 모델 지원이 라이브러리 릴리즈로 자동 전파, 두 가치를 산다고 판단. 자세한 운영 비용은 Risks 표 참고.
- **Maintainer note:** <!-- TODO: 본인 한 줄 voice 로 -->

---

### Synthesis — Go 인터페이스 시그니처

```go
package provider

import (
    "context"
    "encoding/json"
    "errors"
    "time"
)

// Provider abstracts a chat-completion vendor (OpenAI, Anthropic, ...).
type Provider interface {
    Chat(ctx context.Context, req ChatRequest) (ChatResponse, error)
    Name() string
    SupportsModel(model string) bool
}

type Role string

const (
    RoleUser      Role = "user"
    RoleAssistant Role = "assistant"
)

type Message struct {
    Role    Role
    Content string
}

type ChatRequest struct {
    Model     string    // vendor model id, e.g. "gpt-4o", "claude-opus-4-7"
    System    string    // optional; adapters place per-vendor (Anthropic top-level, OpenAI as first message)
    Messages  []Message // alternating user/assistant; do NOT include system here
    MaxTokens *int      // nil = adapter-configured default.
                        //   OpenAI adapter:    nil passed through (vendor default applies).
                        //   Anthropic adapter: nil substitutes adapter-configured fallback;
                        //                       no fallback configured → ErrInvalidInput.
}

type FinishReason string

const (
    FinishStop          FinishReason = "stop"           // natural end_turn / stop
    FinishLength        FinishReason = "length"          // max_tokens reached
    FinishContentFilter FinishReason = "content_filter"  // safety block (OpenAI; Anthropic has no direct equivalent)
    FinishStopSequence  FinishReason = "stop_sequence"   // hit a configured stop sequence
    FinishToolUse       FinishReason = "tool_use"        // model wants to invoke a tool
)

type Usage struct {
    InputTokens  int
    OutputTokens int
}

type ChatResponse struct {
    Content      string          // joined plain text; non-text blocks surfaced via Raw
    FinishReason FinishReason    // gateway-normalized vocabulary (vendor reasons mapped here)
    Usage        Usage
    Raw          json.RawMessage // original vendor response body for debugging / future tool_use unwrapping
}

// ErrorType is a gateway-normalized error category. Provider adapters map
// vendor error types into one of these so the router decides without string matching.
type ErrorType string

const (
    ErrorTypeRateLimit    ErrorType = "rate_limit"
    ErrorTypeOverloaded   ErrorType = "overloaded"
    ErrorTypeAuth         ErrorType = "auth"
    ErrorTypePermission   ErrorType = "permission"
    ErrorTypeInvalidInput ErrorType = "invalid_input"
    ErrorTypeNotFound     ErrorType = "not_found"
    ErrorTypeServer       ErrorType = "server"
    ErrorTypeTimeout      ErrorType = "timeout"
    ErrorTypeUnknown      ErrorType = "unknown"
)

// ProviderError carries enough information for the router to choose retry,
// failover, or abort without parsing vendor-specific strings.
type ProviderError struct {
    Type       ErrorType
    Retriable  bool
    RetryAfter *time.Duration // vendor-provided backoff hint (e.g. Retry-After header); nil if absent
    StatusCode int
    Message    string
    Wrapped    error

    // vendor is unexported on purpose: read via Vendor() to prevent
    // `if err.Vendor == "openai"` abstraction leaks at call sites.
    vendor string
}

func (e *ProviderError) Vendor() string { return e.vendor }

func (e *ProviderError) Error() string {
    return e.vendor + ": " + string(e.Type) + ": " + e.Message
}

func (e *ProviderError) Unwrap() error { return e.Wrapped }

// NewProviderError is the constructor adapter packages (pkg/provider/openai,
// pkg/provider/anthropic, ...) MUST use — the unexported `vendor` field cannot
// be set via struct literal from outside this package.
func NewProviderError(vendor string, t ErrorType, statusCode int, retriable bool, msg string, wrapped error) *ProviderError {
    return &ProviderError{
        Type:       t,
        Retriable:  retriable,
        StatusCode: statusCode,
        Message:    msg,
        Wrapped:    wrapped,
        vendor:     vendor,
    }
}

// WithRetryAfter attaches a vendor-provided backoff hint and returns the
// receiver so adapters can chain: NewProviderError(...).WithRetryAfter(d).
// Only set when the vendor response actually carries Retry-After.
func (e *ProviderError) WithRetryAfter(d time.Duration) *ProviderError {
    e.RetryAfter = &d
    return e
}

// Is allows errors.Is(err, ErrRateLimited) etc.
func (e *ProviderError) Is(target error) bool {
    switch target {
    case ErrRateLimited:
        return e.Type == ErrorTypeRateLimit
    case ErrOverloaded:
        return e.Type == ErrorTypeOverloaded
    case ErrAuthFailed:
        return e.Type == ErrorTypeAuth
    case ErrTimeout:
        return e.Type == ErrorTypeTimeout
    }
    return false
}

// Sentinel errors for ergonomic checks at call sites and in the router.
// Only router-critical categories get sentinels (see Q5 reasoning).
// Other categories (Permission, InvalidInput, NotFound, Server, Unknown) are
// reached via `errors.As(err, &pe)` then inspect `pe.Type`.
var (
    ErrRateLimited = errors.New("provider: rate limited")
    ErrOverloaded  = errors.New("provider: overloaded")
    ErrAuthFailed  = errors.New("provider: authentication failed")
    ErrTimeout     = errors.New("provider: timeout")
)

// Request id propagation does NOT live in this package — see Q6.
// Helper and key are in `internal/requestctx`; external entry point is
// `gateway.WithRequestID(ctx, id)`. Keeping them out of pkg/provider
// preserves the "pkg/* must not import pkg/*" rule in architecture.md.
```

### Request id helper (`internal/requestctx` + `gateway` wrapper)

`pkg/logging`, `pkg/router`, `pkg/metrics` all need to read the request id, so the key/helper cannot live in any `pkg/*`. It is hoisted to `internal/`, then the composition root re-exposes a single external entry point.

```go
// internal/requestctx/requestctx.go
package requestctx

import "context"

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
```

```go
// gateway/requestid.go
package gateway

import (
    "context"

    "github.com/thedev-junyoung/thedev-junyoung-go-llm-gateway/internal/requestctx"
)

// WithRequestID attaches a request id to ctx. Middleware or callers use this
// to correlate logs, metrics, and provider failover traces.
func WithRequestID(ctx context.Context, id string) context.Context {
    return requestctx.With(ctx, id)
}
```

> **Note for maintainer:** 이 시그니처가 `README.md` 의 "Quick example" 과 일치하는지 본인 review 시 확인. 불일치 시 README 도 같은 PR 또는 후속 PR 로 업데이트.

---

## Alternatives Considered

### Alt 1 — Q5 에서 단일 `error` (옵션 A)

- **장점:** 인터페이스 작음. Go 의 단순한 error 모델 유지.
- **단점:** Router 가 retry/failover 판단하려면 매번 string match 또는 `errors.As(*url.Error)` 같은 hack. 두 벤더의 error 어휘가 달라서 정확도 떨어짐.
- **안 선택한 이유:** Router 가 본 라이브러리의 핵심 가치 (failover). 그 핵심을 약하게 만드는 결정은 v0.1 출시 의미를 흐림.

### Alt 2 — Q2 에서 OpenAI 모양 (옵션 A: 단일 Messages 슬라이스에 system role)

- **장점:** OpenAI 익숙한 사용자에게 자연스러움. Anthropic 어댑터가 messages 에서 system role 만 추출해서 top-level 로 옮기면 됨.
- **단점:** OpenAI 는 multiple system messages 가능하지만 Anthropic 은 단일 `system` 필드. concat 으로 처리하면 의미 손실 (예: "You are X" 와 "Respond in JSON" 을 합치면 두 번째 지시의 우선순위가 불명확).
- **안 선택한 이유:** Lossy 변환을 외부 API 가 강요하는 건 인터페이스 잘못. 더 엄격한 모양(B) 을 노출하면 양 방향 변환 lossless.

### Alt 3 — Q3 에서 `Raw` 없음 (옵션 A)

- **장점:** ChatResponse 가 더 작음, 메모리 효율.
- **단점:** tool_use, multi-modal content blocks 같은 케이스에서 사용자가 원본 못 봄. 디버깅 시 "gateway 가 빠뜨렸나, vendor 가 안 보냈나" 판단 불가.
- **안 선택한 이유:** Production gateway 의 가치 중 하나가 observability. Raw 보존은 그 가치의 기초.

### Alt 4 — Q7 에서 동적 모델 검증 (vendor API 호출 + 캐시)

- **장점:** `SupportsModel()` 가 hard-coded 목록이 아니라 vendor 의 진실 (`GET /v1/models`) 을 묻는다. 신규 모델 자동 지원, deprecate 자동 반영.
- **단점:** Anthropic 은 public models 엔드포인트 없음 (2026-05 기준). 두 벤더 동일 패턴이 안 됨. 또한 cold start 마다 외부 호출은 SupportsModel 의 의도(빠른 라우팅 판단)와 어긋남.
- **안 선택한 이유:** 벤더 불일치 + 호출 부담. 두 벤더 모두 models endpoint 갖추는 시점에 ADR 재방문.

### Alt 5 — Q5 에서 `ProviderError.Vendor` 를 exported public field

- **장점:** 로깅 코드가 `err.Vendor` 로 바로 접근 가능. accessor 메서드 호출 없음.
- **단점:** Negative 에서 경고한 `if err.Vendor == "openai"` 안티패턴을 컴파일 타임에 막을 수 없음. 사용자 코드가 벤더 분기를 도입하면 abstraction leak.
- **안 선택한 이유:** 본 ADR 이 스스로 경고한 패턴을 구조적으로 열어두는 건 self-defeating. unexported + accessor 가 약간의 ergonomics 비용으로 안티패턴을 컴파일 차단. ergonomics 손실은 `errors.As + ErrorType` 우선 사용 가이드로 보완.

---

## Consequences

### Positive

- Router 가 error 분류만 보고 retry/failover/abort 결정 가능 — 핵심 가치 정상 동작
- Provider 인터페이스가 thin (3 메서드) — 새 벤더 추가 시 구현 비용 최소
- Anthropic 의 system 분리 모양을 외부 노출해서 양 벤더 어댑터의 텍스트 응답 path 가 lossless (multi-modal content 는 Negative/Risk 참고)
- Sentinel 에러로 사용자 코드가 `errors.Is(err, provider.ErrRateLimited)` 같이 표준 Go 패턴 사용 가능

### Negative

- Stream/Embed 가 v0.1 에 없음 → 사용자가 streaming 필요하면 vendor SDK 직접 호출해야 함 (gateway 우회). v0.2 에서 인터페이스에 추가 → breaking change 1회.
- `Raw json.RawMessage` 노출은 vendor SDK 업데이트 시 응답 schema 변경 가능성 — 사용자가 직접 파싱하면 깨질 수 있음. README/doc 에 "Raw 는 debug 용, production 의존 금지" 명시 필요.
- `ProviderError.Vendor()` 가 read-only accessor 라 직접 비교(`err.Vendor == "openai"`)는 컴파일 차단. 그러나 `if err.Vendor() == "openai"` 식으로 우회 가능 — 안티패턴이지만 doc 가이드만 가능. errors.Is + ErrorType 우선 사용 권장.
- `ProviderError.Error()` 가 `Message` (vendor 원문) 를 포함 → HTTP 응답 body 로 그대로 흘러가면 vendor 내부 상태 노출 가능. **Gateway layer 는 외부 클라이언트에 `Error()` 문자열을 직접 노출하지 말 것; `ErrorType` 만 매핑해서 반환** (예: HTTP 코드 + sanitized message).
- `Content string` 단일 필드 — image / document / tool_use 같은 multi-modal block 표현 불가. 해당 케이스는 `Raw` 로 fallback 강제 → 사용자 코드가 raw schema 학습해야 함. v0.2+ 의 typed content 모델 진화 시 breaking change 가능성.

### Risks

| Risk | Mitigation |
|---|---|
| Anthropic 이 `system` 필드 정책 변경 (multi-system 허용 등) | 어댑터 내부만 수정, 인터페이스 영향 없음 |
| OpenAI 가 응답 shape 을 array of blocks 로 통일 (Anthropic 따라) | `Content string` join 로직만 양 어댑터에 추가 |
| sentinel error 가 너무 적음 (실제 운영에서 새 카테고리 필요) | `ErrorType` enum 추가 + 새 sentinel. Binary level non-breaking 이나 사용자가 `switch e.Type` exhaustive 검사하면 새 값이 default 로 떨어짐 (Go 컴파일러는 string-base enum exhaustiveness 미체크) — README/doc 에 "default 절 필수" 가이드 추가 필요 |
| 사용자가 `Raw` 의 schema 에 의존 | doc 강조, 향후 ADR 에서 `Raw` 를 typed union 으로 진화 검토 |
| `SupportsModel()` 의 모델 목록 유지보수 비용 (벤더 deprecate / 신규 출시 / 이름 변경) | 신규 모델 출시 시 minor release 로 모델 목록 PR. 긴급 시 사용자가 fork 또는 ADR-003 에서 "config override" 메커니즘 도입 검토 |
| `vendor` unexported 이지만 `Vendor()` accessor 로 우회 가능 | doc 가이드 + 코드 리뷰. 구조적 차단은 컴파일타임 직접 비교만; 우회 의도가 있는 사용자는 못 막음 |

---

## Open Questions

- [ ] Streaming 인터페이스 (`ChatStream`?) — v0.2 ADR. blocking 호출과 동일 결과를 streaming 으로도 보장하는 design.
- [ ] `tool_calls` / `function_calls` 지원 시점 — Provider 인터페이스에 별도 메서드 vs `ChatRequest` 확장 필드.
- [ ] Gemini 추가 시 (v0.2+) `ChatRequest` 어떤 필드가 추가될지 — 셋째 벤더가 패턴 검증.
- [ ] Embedding 인터페이스 — `pkg/embedding/` 별 패키지로 빼는 게 모듈 경계상 맞을지 (Provider 와 분리).
- [ ] `Content string` 의 multi-modal 표현 한계. v0.2+ 에서 `Content []Block` (Anthropic 스타일) 또는 `Content ContentUnion` 같은 typed union 으로 진화할지. v0.1 의 "lossless 양방향 변환" 주장은 텍스트-only 케이스 한정 — 멀티모달 도입 시 첫 가정이 무너지므로 v0.2 ADR 의 첫 항목.
- [ ] Sentinel 4개 → N개 확장 트리거. 어떤 사용 사례가 등장하면 `ErrPermission` / `ErrInvalidInput` / `ErrNotFound` / `ErrServer` 를 sentinel 로 승격할지 기준.
