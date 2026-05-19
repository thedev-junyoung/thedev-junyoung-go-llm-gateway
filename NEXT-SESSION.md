# Next Session — Cold Start Guide

새 세션(다른 시간, 다른 에이전트 컨텍스트)에서 이 프로젝트로 들어올 때 첫 5분 안에 읽을 것.

---

## 이 프로젝트는 뭔가
**`go-llm-gateway`** — LiteLLM(Python, 12k stars)의 Go 대안 OSS.

- **목적:** 학습 + 포트폴리오 + (운 좋으면) 수익. 메인 자산은 코드 + ADR + 블로그.
- **트랙:** 에이전트 드리븐 개발 + ADR 이력화 + 메타 글쓰기.
- **타임라인:** 2026-05-19 ~ 2026-07-06 (6주). 본업 병행.
- **타겟 사용자:** Go 백엔드 팀이 LLM 호출하는 production 환경.

## 왜 이걸 하기로 했나
1. **분산락 + 캐싱 + TDD** 학습 강점을 정면 활용
2. **Go 생태계 빈자리** — Python엔 LiteLLM/Portkey/Helicone, Go엔 없음
3. **글로벌 채용 어필** — Anthropic / Vercel / Modal 등의 정확한 스택
4. **2026년 트렌드** — 에이전트 드리븐 개발 워크플로우 자체가 글감

## 첫 5분 읽기 순서
1. `README.md` — 프로젝트 한 줄 이해
2. `docs/roadmap.md` — 지금 어느 주차인가
3. `docs/design/v0.1-scope.md` — 무엇을 만드는가
4. `docs/adr/0001-why-go-llm-gateway.md` — 왜 만드는가
5. `docs/workflow/agent-driven-development.md` — **어떻게 만드는가 (가장 중요)**

## 절대 잊으면 안 되는 룰
> **타이핑은 위임 OK. 사고는 위임 NOT OK.**

매 PR 머지 전 단 하나의 셀프 체크:
> "이 코드, 6개월 뒤에 술자리에서 후배에게 1시간 설명할 수 있는가?"

YES면 머지, NO면 강한 질문 → 이해 → 머지.

자세한 룰은 `docs/workflow/agent-driven-development.md`.

## 다음 즉시 행동 (Week 0 잔여 작업)
1. GitHub repo 생성 (`go-llm-gateway`, public, MIT)
2. `go mod init github.com/thedev-junyoung/thedev-junyoung-go-llm-gateway`
3. `.github/workflows/ci.yml` 골격 (test + lint)
4. README의 "Quick example" 코드가 진짜 호출 가능한 API가 되도록 ADR-002 작성
5. ADR-002 작성 — Provider 인터페이스 설계 (본인 손으로 1차 초안)

## 첫 ADR 작성 시 셀프 질문 (강한 질문 예시)
- `Provider` 인터페이스에 `Chat()` 하나만? `Stream()`, `Embed()`도? — v0.1에선 `Chat()`만, 이유는?
- 모델명을 `string`으로? 아니면 `ModelID` 타입? — 트레이드오프는?
- 에러는 단일 `error` 타입? 아니면 `*ProviderError`로 구분? — failover 라우터가 뭐를 보고 trigger 결정?
- Streaming은 v0.1 out of scope인데, 인터페이스를 미리 streaming-ready로 둘지 vs 나중에 breaking change 감수할지?

## 작업 시 사용 가능 도구
- Claude Code (이 환경) — 코드 페어, 리뷰, 강한 질문 spar
- Cursor / Gemini CLI — 백업
- **금지:** "에이전트야 다 짜줘" 식 위임 (룰 위배)

## 회고 사이클
- **매 주 금요일:** 그 주 ADR + 블로그 출간 현황 점검
- **2주마다:** 일정 vs 실제 비교, 스코프 조정 검토
- **Week 6 끝:** 회고 1편 작성 (`docs/retrospective.md`)
