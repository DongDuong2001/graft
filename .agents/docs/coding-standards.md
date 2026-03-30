# Coding Standards for AI Agents

To maintain clean code and avoid subtle bugs, follow these guidelines. **For a single entry point** (quality + efficiency + testing rules for this repo), read **`.agents/SKILL.md`**. **For required tests per feature**, read **`.agents/FEATURE-TEST-MATRIX.md`**.

## 1. Architecture and layout

- Follow standard Go layout (`cmd/`, `internal/`).
- Prefer separation of concerns: HTTP handlers parse/validate, call storage and domain helpers, then write responses. Extract services when handlers grow beyond ~80 lines of logic.
- Do not introduce global mutable state or `init()` side effects beyond driver registration.

## 2. Dependency injection

- Pass dependencies via constructors (`NewHandler`, `NewForwarder`, etc.).
- No hidden singletons for DB or config.

## 3. Error handling

- Always check `if err != nil`.
- Avoid `_` on errors unless justified in a short comment.
- Wrap with `fmt.Errorf("context: %w", err)` when crossing package boundaries.

## 4. Testing (required before public)

- **Every feature area** listed in **`.agents/FEATURE-TEST-MATRIX.md`** must have `_test.go` coverage; add table-driven tests when there are multiple cases.
- Mock outbound HTTP with `httptest.Server`; mock time only when tests would be flaky.
- **Integration**: keep `internal/integration` aligned with the main HTTP wiring when routes or auth change.
- SQLite tests require **CGO**; skipped tests on a dev laptop without CGO are OK only if **CI runs with CGO=1** before release.

## 5. Simplicity and readability

- Prefer explicit names and small functions.
- Comments explain *why*, not *what*.

## 6. Concurrency

- Goroutines must have a clear lifecycle (no unbounded goroutines per request).
- Protect shared maps with `sync.Mutex` or design away shared mutation.

## Strict enforcement

Before declaring a task complete: **`go fmt ./...`**, **`go vet ./...`**, **`go test ./...`**. Follow **`.agents/workflows/test-and-lint.md`** for release-level checks (race + CGO).
