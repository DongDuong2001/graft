---
name: graft-webhook-bridge
description: >-
  Project skill for Graft (self-hosted webhook-to-anything bridge). Use when
  editing this repo to enforce quality, security, tests, and efficient agent
  workflows before changes ship or go public.
---

# Graft — Agent Skill

## Purpose

Graft receives signed (optional) webhooks on `/hook/…`, transforms JSON with Go `text/template`, and forwards to destination URLs. Admin APIs live under `/api/v1/` with API key auth. Agents must keep the codebase safe for small teams running on the public internet and **must not merge behavior without tests** for the touched surface.

## When this skill applies

Use for any task in this repository: new features, bug fixes, refactors, dependency bumps, or release prep.

## Efficiency (how to work this codebase)

1. **Scope** — Change only packages that the task requires (`internal/httpapi`, `internal/connectors`, `internal/storage`, `internal/router`, etc.). Avoid drive-by refactors.
2. **Discover first** — Read the handler and storage path for the feature before editing; mirror existing patterns (constructors, error messages, JSON field names).
3. **Single verification pass** — After edits, run `go fmt ./...`, `go vet ./...`, and `go test ./...` once at the end of the task batch (not after every micro-edit).
4. **CGO** — `github.com/mattn/go-sqlite3` needs **CGO** for real DB tests. Local runs with `CGO_ENABLED=0` will **skip** SQLite-backed tests; **CI / pre-release must use CGO=1** (see workflow doc).
5. **Dependencies** — Add third-party modules only when necessary; prefer the standard library.

## Quality bar (non-negotiable)

1. **Errors** — Check `err != nil`; wrap with `%w` where useful. Do not return stack traces or internal details in HTTP responses.
2. **Security** — Follow `.agents/security-guidelines.md` (admin auth, signature verification, encryption at rest, rate limits, body limits, timeouts).
3. **Handlers** — Keep HTTP thin: validate input, call storage, `connectors`, `transformer`, etc., then write status and JSON. Prefer small helpers over duplicating security checks.
4. **Logging** — Do not log raw webhook bodies or decrypted secrets.

## Testing contract (before public / every feature)

See **`.agents/docs/FEATURE-TEST-MATRIX.md`** for the canonical map of **feature → package → test file**.

Rules:

- **Every new feature or behavior change** must include or update **unit tests** in the listed package (table-driven where there are multiple cases).
- **Integration**: `internal/integration` covers admin + webhook + forward end-to-end; run with CGO when validating releases.
- If SQLite is unavailable in an environment, skipped tests are acceptable **only** for developer machines; **release pipelines must run with CGO** so storage, admin, and webhook tests execute.

## Mandatory completion checklist

1. `go fmt ./...`
2. `go vet ./...`
3. `go test ./...` (local; CGO optional)
4. Before **public release** or **CI merge**: `go test -race ./...` with **CGO_ENABLED=1** and a C toolchain (see `.agents/workflows/test-and-lint.md`).

## Cross-references

| Document | Role |
|----------|------|
| `.agents/docs/coding-standards.md` | Architecture, DI, concurrency |
| `.agents/docs/security-guidelines.md` | Security requirements |
| `.agents/workflows/test-and-lint.md` | Exact commands and CI notes |
| `.agents/docs/FEATURE-TEST-MATRIX.md` | Required tests per area |
