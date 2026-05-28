# Roadmap

## Timeline: 2026-05-19 ~ 2026-07-06 (약 6주)

본업과 병행. 매주 평균 10-15시간 가정.

---

## Week 0 — 기초 (~ 2026-05-25, **실제 종료: 2026-05-27**)
**모드: 본인 100%. 에이전트는 조언자만.**

- [x] 작명 결정: `go-llm-gateway`
- [x] 문서 골격 (README, design, ADR-001, workflow, roadmap)
- [x] GitHub repo 생성 (public, MIT license, `.github/workflows/ci.yml` 골격) — 2026-05-19
- [x] `go mod init github.com/thedev-junyoung/thedev-junyoung-go-llm-gateway` — 2026-05-20
- [x] CI 골격: `go test ./...` + `golangci-lint` + Dependabot (`#20, #30, #32`)
- [x] README RDD — API 가 어떻게 쓰일지 보이는 코드 예시 완성 — 2026-05-20
- [ ] ADR-002 — Provider 인터페이스 설계 결정 (⏳ PR `#34`, 본인 voice 채우는 중)

**Deliverable:** repo public + README + ADR 2편 + CI green
**Status:** ADR-002 Status `Proposed → Accepted` 전환 + PR `#34` 머지 완료 시 Week 0 공식 종료. 그 외 작업은 모두 main 머지됨.

---

## Week 1-2 — Provider Abstraction (~ 2026-06-08)
**핵심: 통일된 Provider 인터페이스**

- [ ] `pkg/provider/provider.go` — 인터페이스 정의
- [ ] `pkg/provider/openai/` — Chat Completions 구현
- [ ] `pkg/provider/anthropic/` — Messages API 구현
- [ ] 모델명 매핑 (`gpt-4o` ↔ `claude-3-5-sonnet`, ADR-003)
- [ ] Recorded response 기반 테스트 (go-vcr 또는 자체)
- [ ] ADR-003: 모델명 매핑 전략
- [ ] **블로그 1편 출간:** "Why I'm building a Go alternative to LiteLLM"

**Deliverable:** 두 provider 호출 + 단위 테스트 통과

---

## Week 3 — Failover Routing (~ 2026-06-15)
**핵심: 고루틴 + context 기반 failover**

- [ ] `pkg/router/` 설계
- [ ] Failover 정책 인터페이스 (`Primary`, `Fallbacks`, `Triggers`)
- [ ] Timeout / 5xx / rate-limit 응답 감지
- [ ] Context propagation (deadline, request id)
- [ ] 통합 테스트 (mock provider with fault injection)
- [ ] ADR-004: Failover trigger 조건 + 재시도 정책
- [ ] **블로그 2편 출간:** "Multi-provider failover patterns with goroutines"

**Deliverable:** OpenAI 5xx → Anthropic 자동 라우팅 + metric 기록

---

## Week 4 — Distributed Rate Limit (~ 2026-06-22)
**핵심: 분산락 활용 — 본인 강점 정면**

- [ ] `pkg/ratelimit/` 설계
- [ ] Redis 백엔드 (`SET NX` + Lua script로 원자성)
- [ ] Sliding window 알고리즘 (RPM + TPM)
- [ ] 멀티 인스턴스 부하 테스트 (docker-compose로 3 instance + Redis)
- [ ] ADR-005: Sliding window vs token bucket 선택 이유
- [ ] **블로그 3편 출간:** "Distributed rate limiting for LLM APIs in Go"

**Deliverable:** 멀티 인스턴스 환경에서 rate limit 정확히 동작

---

## Week 5 — Observability (~ 2026-06-29)
**핵심: Prometheus + slog**

- [ ] `pkg/metrics/` — Prometheus collectors
- [ ] `pkg/logging/` — slog handler + request id propagation
- [ ] 비용 계산 (provider별 단가 테이블, 환경변수로 override 가능)
- [ ] Grafana 대시보드 JSON 예시 (`examples/grafana/`)
- [ ] ADR-006: Metric naming convention
- [ ] **블로그 4편 출간:** "Observability for LLM workloads: what to measure"

**Deliverable:** Prometheus 스크랩 + Grafana 대시보드

---

## Week 6 — v0.1 Release (~ 2026-07-06)
**핵심: 출시 + 메타 자산화**

- [ ] `examples/` — 3-5개 시나리오 (기본 사용, failover, rate limit, 비용 로깅)
- [ ] README 영문 마무리 + 비교 표 (vs LiteLLM)
- [ ] `CHANGELOG.md`
- [ ] v0.1.0 태그 + GitHub Release
- [ ] HN / Reddit r/golang / Twitter 공유
- [ ] **블로그 5편 출간:** "Agent-driven OSS development: my workflow & guardrails"

**Deliverable:** v0.1.0 출시 + 블로그 5편 누적

---

## 마일스톤별 누적 자산

| Week | ADR 누적 | 블로그 누적 | 대략 LOC |
|---|---|---|---|
| 0 | 2 | 0 | 0 |
| 2 | 3 | 1 | ~1,000 |
| 3 | 4 | 2 | ~2,000 |
| 4 | 5 | 3 | ~3,000 |
| 5 | 6 | 4 | ~3,500 |
| 6 | 6+ | 5 | ~4,000 |

**6주 후 면접 자산:**
- ADR 6+편 — 의사결정 깊이 입증
- 블로그 5편 — 사고 흐름 입증
- v0.1 OSS — 실행력 입증
- Agent-driven 워크플로우 — 2026년 트렌드 어필

---

## Out of Roadmap (v0.2+)
- Semantic cache (옵션 모듈)
- Streaming responses
- HTTP proxy mode
- Gemini / Bedrock / Vertex
- Cost budget guard
- A/B 라우팅
- Hosted SaaS 검토

---

## Honest Stop Criteria

루틴이 무너질 때 솔직하게 멈추는 기준 — vibrocoding 방지.

- **4주차에 v0.1 50% 미만** → 스코프 축소 (Anthropic 빼고 OpenAI만 / 또는 rate limit 빼고)
- **본업이 빡세짐** → 일정 연장 OK. 룰 위배(에이전트 자동 머지) 금지.
- **블로그 페이스 못 따라감** → 코드 멈추고 글부터. 자산 우선순위는 글 > 코드.
- **흥미 잃음** → 솔직히 인정하고 회고 1편 쓰고 종료. 회고도 자산.

멈춤이 실패가 아니라 **회고하지 않는 멈춤**이 실패다.
