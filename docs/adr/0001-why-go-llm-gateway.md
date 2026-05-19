# ADR-001 — Why a Go LLM Gateway

- **Status:** Accepted
- **Date:** 2026-05-19
- **Author:** Junyoung

## Context

LLM API를 production에서 호출하는 Go 서비스가 빠르게 늘고 있다. 그러나 Go 생태계에는 multi-provider gateway가 사실상 없다.

### 시장 상황 (2026-05 기준)
- **Python:** LiteLLM (12k+ stars), Portkey (~5k), Helicone (~3k) — 모두 활발
- **Go:** 정공법 라이브러리 부재
  - ByteDance `eino` (~5k) — 프레임워크지만 gateway 아님
  - `langchaingo` — LangChain 포팅, gateway 기능 약함
  - swarmgo, agency — 소규모, 미완성

### 각 팀이 반복 구현하는 것
1. Provider 추상화 (OpenAI / Anthropic / Gemini / Bedrock)
2. Failover (한 provider 다운 시 다른 데로)
3. Rate limit (API key 보호, multi-instance)
4. Token 사용량 / 비용 / latency 메트릭

### 지금 시작하는 이유
- Anthropic Go SDK 공식 출시 + OpenAI Go SDK 안정화
- Go의 동시성/타임아웃 primitive가 failover·rate limit 구현에 자연 핏
- Python wrapper로 우회 불가능한 사용 사례: 인프라/플랫폼 팀의 Go 백엔드

## Decision

**Go로 multi-provider LLM gateway 라이브러리를 만든다.**

차별점:
1. **Go-native** — Python wrapper 없이 단일 바이너리/import 가능
2. **인프라 primitive 강조** — 분산락 기반 rate limit, 고루틴 failover
3. **Observability first-class** — Prometheus + slog 기본 내장
4. **Library first, proxy second** — Go 서비스에 import 하는 게 우선, HTTP proxy는 v0.2

## Alternatives Considered

### Alt 1 — Go용 LangChain 클론 (프레임워크)
- ❌ eino, langchaingo가 이미 자리 잡음. 레드오션.
- ❌ 프레임워크는 도입 비용이 큼. Gateway는 사이드카로 import 가능해 진입장벽 낮음.

### Alt 2 — Semantic cache 단독 라이브러리
- ❌ 사용 사례 좁음 (FAQ 챗봇 정도)
- ❌ 정확도 리스크 큼 — 임베딩 유사도가 의미 동일성 보장 못 함
- ❌ 벤더 prompt caching (OpenAI 50%, Anthropic 90%, Gemini context cache)이 정확 매칭 + 큰 할인 제공 → semantic cache 매력 감소

### Alt 3 — B2B SaaS로 빌드 (Helicone 모델)
- ❌ 사이드 프로젝트로는 영업·CS 부담 큼
- ❌ "운영 30분/일" 제약 위배
- 🔄 OSS로 시작 → 검증되면 hosted version으로 확장 옵션 보존

### Alt 4 — HTTP proxy first (LiteLLM의 main 모드)
- ❌ Go 사용자 다수는 라이브러리 import 선호 (단일 바이너리, deploy 단순화)
- 🔄 HTTP proxy는 v0.2에서 라이브러리 위에 얇은 wrapper로 추가

## Consequences

### Positive
- 분산락·캐싱 학습 강점을 정면 활용
- ADR/포스트모템 글감 풍부 (multi-provider failover, distributed rate limit 등)
- 글로벌 채용 시그널 (Anthropic, Vercel, Modal, Encore 등)
- v0.2에서 hosted SaaS 확장 옵션 열어둠

### Negative
- 6주 진지한 시간 투자 필요
- LLM provider API 변동 추적 부담 (특히 Anthropic은 잦음)
- Go LLM 사용자 풀 자체가 Python보다 작음 → 초기 사용자 확보 느림

### Risks & Mitigation
| 리스크 | 완화 |
|---|---|
| Provider API breaking change | Contract test 자동화 (recorded responses + nightly real-call) |
| Python 진영 압도적 mindshare | Go 백엔드 팀으로 타겟 좁히기, Go 컨퍼런스 발표 노림 |
| AI assist 의존도 과다 → 학습 0 | `docs/workflow/agent-driven-development.md` 룰화 |
| 6주 일정 미달 | Stop criteria 명시 (roadmap.md), 스코프 축소 옵션 보유 |

## Open Questions
- [ ] HTTP proxy mode v0.1 포함? → **No, v0.2.**
- [ ] Anthropic Bedrock vs Direct API? → **v0.1은 Direct API only.**
- [ ] License: MIT vs Apache 2.0? → **MIT** (덜 까다로움, contributor 진입 쉬움).
- [ ] 모듈 path: `github.com/junyoung-acloset/go-llm-gateway` vs 다른 org? → 다음 ADR.
