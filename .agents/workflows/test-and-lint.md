---
description: Automated testing and static analysis — required before completing code changes; stricter gate before public release.
---

# Test and Lint Workflow

Run these commands after substantive edits. If any step fails, fix the code before finishing the task.

## 1. Format

```bash
go fmt ./...
```

## 2. Static analysis

```bash
go vet ./...
```

## 3. Unit and integration tests

```bash
go test ./...
```

**CGO and SQLite:** `github.com/mattn/go-sqlite3` is used with CGO. With `CGO_ENABLED=0` (common on some Windows setups without a C compiler), tests in `internal/storage`, `internal/httpapi`, and `internal/integration` may **skip** after opening the DB. That is acceptable for quick local checks.

## 4. Pre-public / CI gate (recommended)

Before tagging a release or publishing images for general use:

```bash
export CGO_ENABLED=1   # Linux/macOS with gcc/clang; Windows with a working C toolchain
go test -v -race ./...
```

- **Race detector** requires CGO on many platforms.
- If `-race` is not available in an environment, still run **`go test ./...` with CGO=1** so no storage or handler tests are skipped.

## 5. Optional coverage

```bash
go test ./... -coverprofile=coverage.out
go tool cover -func=coverage.out
```

Use coverage to find gaps; the **feature matrix** in `.agents/FEATURE-TEST-MATRIX.md` is the source of truth for what must exist.

## Strict enforcement

Failing **fmt**, **vet**, or **test** (with CGO enabled in CI) is a blocker for merge and release.
