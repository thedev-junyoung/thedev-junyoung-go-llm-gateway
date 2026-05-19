---
name: issue-driven-pr
description: Use BEFORE touching code in this repo. Enforces the project rule that every change ships as issue → branch → PR → CodeRabbit review → squash-merge to main. Direct commits to main are forbidden except for the one-time bootstrap.
---

# Issue-Driven PR Workflow (`thedev-junyoung-go-llm-gateway`)

> **The rule:** No change lands on `main` without an issue, a PR, and a passing CodeRabbit review. Bootstrap is the only exception (documented in commit history).

This skill is **rigid** — follow the checklist exactly. The discipline is the point: every change becomes an ADR-shaped paper trail (issue = problem, PR = solution, review = critique).

## When to use

Invoke this skill whenever the user asks you to:
- Implement a feature, fix, or refactor
- Update docs, ADRs, or CI
- Add tests, examples, or tooling
- Anything that produces a diff to be merged

If you find yourself about to run `git commit` on `main`, **stop** and start from step 1.

## The 7-step checklist

Create a TodoWrite item per step and check them off as you go.

### 1. Confirm scope with the user
- One sentence: what's the change, and what's the *intent* behind it?
- If the change crosses module boundaries (per `docs/design/v0.1-scope.md`) or implies a non-obvious decision, surface that — it may need an ADR before code.

### 2. Create the GitHub issue
- `gh issue create --title "<type>: <short imperative>" --body "..."`
- Issue body must include:
  - **Problem** — what's broken / missing / unclear
  - **Proposed approach** — bullets, not paragraphs
  - **Acceptance criteria** — checkboxes a reviewer can tick
  - **Out of scope** — what this issue intentionally does *not* touch
- Title prefix matches Conventional Commits: `feat:`, `fix:`, `chore:`, `docs:`, `refactor:`, `test:`, `perf:`, `ci:`, `build:`.
- Capture the issue number — every downstream artifact references it.

### 3. Branch from a clean main
```bash
git switch main
git pull --ff-only
git switch -c <type>/<issue-num>-<slug>     # feat/12-provider-interface
```
- Branch name: `<type>/<issue-num>-<kebab-slug>`. Slug ≤ 4 words.
- Never branch off another feature branch unless the user explicitly says so.

### 4. Commit on the branch
- **Conventional Commits** title: `<type>(<scope>)?: <imperative>`. Scope optional, lowercase.
- Stage explicit paths (`git add path/to/file`). Never `git add .` / `git add -A`.
- Never `--no-verify`. If a hook fails, fix the root cause and create a **new** commit (do not `--amend`).
- Keep commits atomic — one logical change per commit. Multi-step work = multi-commit PR.
- Every commit ends with the `Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>` trailer.

### 5. Push + open PR
```bash
git push -u origin <branch>
gh pr create \
  --base main \
  --title "<same as issue or refined>" \
  --body "Closes #<issue-num>

## Summary
- ...

## Test plan
- [ ] ...

## Self-check
- [ ] '6개월 뒤 술자리에서 후배에게 1시간 설명할 수 있는가?' — YES"
```
- `Closes #N` is mandatory — the issue auto-closes on merge.
- The self-check line is the project's merge gate (`docs/workflow/agent-driven-development.md`). If the answer is NO, push more commits with explanation or ask the user a strong question first.

### 6. Wait for CodeRabbit + CI
- **CodeRabbit** posts a review automatically (requires the CodeRabbit GitHub App installed on the repo — see setup section below). Treat its comments as a first-pass reviewer: address them, push fixups, mark resolved.
- **CI** (`.github/workflows/ci.yml`) must be green: `go build`, `go test -race`, `golangci-lint`.
- Do not merge until **both** are clean. If CodeRabbit raises something you disagree with, reply with reasoning — don't silently dismiss.
- Use `superpowers:receiving-code-review` skill if any feedback feels unclear or technically questionable.

### 7. Squash-merge to main
```bash
gh pr merge <pr-num> --squash --delete-branch
```
- **Squash** is the default — keeps `main` history linear and one-commit-per-issue.
- Squash commit message: use the PR title and body (PR description becomes the body of the squash commit).
- Delete the branch after merge.
- Pull `main` immediately: `git switch main && git pull --ff-only`.

## Hard rules (no exceptions)

| ❌ Never | ✅ Always |
|---|---|
| Push to `main` directly | Push to feature branch, PR to main |
| Merge without CodeRabbit review | Wait for CodeRabbit comment (or `LGTM`) |
| Merge with red CI | Fix CI, push fixup, re-run |
| `git add .` / `git add -A` | `git add <explicit-path>` |
| `--no-verify` to skip hooks | Fix the underlying lint/format/test failure |
| `--force-push` to `main` | Force-push allowed only on your own feature branch, with `--force-with-lease` |
| Amend pushed commits | New commit; squash on merge handles cleanup |
| Drop CodeRabbit feedback silently | Reply with reasoning, then resolve |

## Bootstrap exception (one-time only)

The very first scaffolding commits (repo init, `go mod init`, CI skeleton, LICENSE, initial docs) **may** land directly on `main` before this skill is enforceable. After the initial push, this skill applies to **every** subsequent change without exception.

Mark the bootstrap commit clearly: `chore: bootstrap repo — workflow enforced from next PR`.

## CodeRabbit setup (one-time, by the human)

1. Visit https://www.coderabbit.ai and install the GitHub App on `thedev-junyoung/thedev-junyoung-go-llm-gateway`.
2. (Optional but recommended) Add `.coderabbit.yaml` at repo root to tune review focus:
   ```yaml
   reviews:
     profile: assertive
     auto_review:
       enabled: true
       drafts: false
     path_instructions:
       - path: "docs/adr/**"
         instructions: "Critique reasoning depth, alternatives considered, and consequences. Be a peer reviewer, not a typo checker."
       - path: "pkg/**/*.go"
         instructions: "Check for idiomatic Go, error wrapping (%w), context propagation, and goroutine leaks."
   ```
3. Enable branch protection on `main` (Settings → Branches):
   - Require a pull request before merging
   - Require status checks: `test`, `lint`
   - Require conversation resolution before merging
   - (Optional) Require review from CodeRabbit

## Anti-patterns (immediately stop if you catch yourself doing these)

- "Just this once, let me commit directly to main" — no. Bootstrap is past.
- "CodeRabbit is being noisy, I'll merge anyway" — no. Reply with reasoning, then merge.
- "I'll batch 5 unrelated changes into one PR to save review cycles" — no. One issue = one concern = one PR.
- "ADR can come later" — no. If the PR encodes a non-obvious decision, the ADR lands in the same PR (or its own preceding PR).

## Why this rule exists

`docs/workflow/agent-driven-development.md`: **타이핑은 위임 OK. 사고는 위임 NOT OK.** Issues force the human to articulate intent before code. PRs force CodeRabbit (and the human) to critique before main. Without this gate, the agent's typing speed becomes a liability instead of an asset.
