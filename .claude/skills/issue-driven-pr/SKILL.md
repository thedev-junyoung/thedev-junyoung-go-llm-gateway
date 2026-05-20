---
name: issue-driven-pr
description: Use BEFORE touching code in this repo. Enforces the project rule that every change ships as issue → branch → PR → AI review (pr-reviewer + adr-critic) + CI → squash-merge to main. Direct commits to main are forbidden except for the documented one-time bootstrap.
---

# Issue-Driven PR Workflow (`thedev-junyoung-go-llm-gateway`)

> **The rule:** No change lands on `main` without an issue, a PR, the in-house AI reviewer's verdict, and green CI. Bootstrap is the only exception (documented in commit history).

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
- [ ] ..."
```
- `Closes #N` is mandatory — the issue auto-closes on merge.
- Keep PR bodies clinical: **Summary** + **Test plan** + (optional) **Out of scope**. Substantive WHY belongs in the commit message body, not in the PR description.
- **Do not** echo the maintainer's private merge gate (`docs/workflow/agent-driven-development.md`) into PR bodies or commit messages. It's a thought tool, not a template item.

### 6. Wait for AI review + CI
- **AI reviewer** runs automatically via `.github/workflows/ai-review.yml`:
  - `code-reviewer` job — broad code review using the `pr-reviewer` skill (`.claude/skills/pr-reviewer/SKILL.md`)
  - `adr-critic` job — only when files under `docs/adr/**` changed; uses the `adr-critic` skill
- **CI** (`.github/workflows/ci.yml`) must be green: `go build`, `go test -race`, `golangci-lint`.
- Do not merge until **both** the AI review verdict and CI are clean. If the reviewer raises something you disagree with, reply on the PR with reasoning — don't silently dismiss.
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
| Merge without an AI review verdict | Wait for the `code-reviewer` (and `adr-critic` if applicable) comment |
| Merge with red CI | Fix CI, push fixup, re-run |
| `git add .` / `git add -A` | `git add <explicit-path>` |
| `--no-verify` to skip hooks | Fix the underlying lint/format/test failure |
| `--force-push` to `main` | Force-push allowed only on your own feature branch, with `--force-with-lease` |
| Amend pushed commits | New commit; squash on merge handles cleanup |
| Drop reviewer feedback silently | Reply with reasoning, then resolve |

## Bootstrap exception (one-time only)

The very first scaffolding commits (repo init, `go mod init`, CI skeleton, LICENSE, initial docs, and the agent-review setup itself) **may** land directly on `main` before this skill is enforceable. After that, this skill applies to **every** subsequent change without exception.

## AI review setup (one-time, by the human)

The in-house "agent team" replaces external paid reviewers. See also the project-memory note `project_ai_pr_review.md`.

Pick **one** of the auth paths. The GitHub App path is preferred — OIDC-based, no secret to rotate.

### Option A — Claude Code GitHub App (preferred)

1. In a Claude Code interactive session, run:
   ```
   /install-github-app
   ```
   This installs the **Claude Code** GitHub App on the chosen repo and wires up OIDC-based auth automatically. No `secrets.*` to manage.
2. Manual fallback: install https://github.com/apps/claude on `thedev-junyoung/thedev-junyoung-go-llm-gateway`.

### Option B — OAuth token secret

Use this only if the App path is unavailable (e.g., org policy disallows App install).

1. Locally: `claude setup-token` → copy the token.
2. Repo Settings → Secrets and variables → Actions → New repository secret
   - Name: `CLAUDE_CODE_OAUTH_TOKEN`
   - Value: the token from step 1.

### Branch protection (after either auth path)

Settings → Branches → Add rule for `main`:
- Require a pull request before merging
- Require status checks: `test`, `lint`, `code-reviewer`
- Require conversation resolution before merging

### Tuning the reviewer

Edit the skill files (`.claude/skills/pr-reviewer/SKILL.md`, `.claude/skills/adr-critic/SKILL.md`) via PR. Skill files are the source of truth; do **not** put review instructions inline in `ai-review.yml`.

## Anti-patterns (immediately stop if you catch yourself doing these)

- "Just this once, let me commit directly to main" — no. Bootstrap is past.
- "The reviewer is being noisy, I'll merge anyway" — no. Reply with reasoning, then merge.
- "I'll batch 5 unrelated changes into one PR to save review cycles" — no. One issue = one concern = one PR.
- "ADR can come later" — no. If the PR encodes a non-obvious decision, the ADR lands in the same PR (or its own preceding PR).
- "Let me prove I considered the merge gate by quoting it in the PR body" — no. The gate is private; quoting it in reviewer-facing artifacts reads as cargo-cult.

## Why this rule exists

`docs/workflow/agent-driven-development.md`: **타이핑은 위임 OK. 사고는 위임 NOT OK.** Issues force the human to articulate intent before code. PRs force the AI reviewer (and the human) to critique before main. Without this gate, the agent's typing speed becomes a liability instead of an asset.
