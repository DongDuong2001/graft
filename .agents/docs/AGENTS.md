# Graft AI Coding Agent Guide

## Default Workflow: PUDO

Use PUDO for meaningful work in this repository:

1. **Plan** - define scope, success criteria, constraints, risks, and the intended implementation path.
2. **Understand** - inspect relevant files before editing and map the affected runtime, storage, API, and test paths.
3. **Develop** - implement in small steps, keep progress visible, and update tests alongside behavior changes.
4. **Optimize** - self-review, run verification, update docs when needed, and summarize the final change.

For tiny tasks, compress the cycle into a brief note. For large tasks, use nested PUDO cycles by feature area. The full workflow lives in `.agents/workflows/pudo.md`.

## Big Picture Architecture
Graft is a lightweight webhook bridge written in Go.
- **Entry Point:** `cmd/graft/main.go` simply calls `internal/app.Run()`.
- **Core Wiring:** `internal/app/app.go` initializes dependencies (DB, Repo, Handlers) and starts the HTTP server.
- **Data Flow (Webhook):**
  1. Ingress at `/hook/{path}` via `internal/httpapi.WebhookHandler`.
  2. Rule lookup in SQLite (`internal/storage`).
  3. Transformation (optional) via `internal/transformer`.
  4. Forwarding via `internal/connectors.HTTPForwarder` (handles retries).
  5. Result stored as `Delivery` in DB.
- **Data Flow (Admin):**
  1. Management API at `/api/v1/` via `internal/httpapi.AdminHandler`.
  2. Protected by `middleware.AdminAuth` and Rate Limiting. Use `AdminAPIKey`.

## Project-Specific Patterns
- **Standard Library:** We use `net/http` and `http.ServeMux`. Avoid adding frameworks like Gin or Echo.
- **Database:**
  - SQLite is the sole engine (`github.com/mattn/go-sqlite3`).
  - Schema is defined in code at `internal/storage/sqlite.go` in `NewSQLiteRepo`.
  - **Constraint:** When modifying the schema, ensure `CREATE TABLE IF NOT EXISTS` is updated.
- **Dependency Injection:** Manual injection in `internal/app/app.go`.
- **Security:**
  - `MasterKey` is used for field-level encryption (e.g., secrets in `internal/storage`).
  - `AdminAPIKey` protects the Admin API.
- **Error Handling:** Wrap errors with `fmt.Errorf("context: %w", err)` to preserve the chain.

## Key Directories
- `internal/models`: Domain structs (`Rule`, `Delivery`). JSON tags define API contracts.
- `internal/storage`: Database implementation. Add new queries here.
- `internal/httpapi`: HTTP Handlers. Separate files for `admin.go` (management) and `webhook.go` (ingestion).
- `internal/connectors`: Outbound HTTP logic (retries, timeouts).

## Developer Workflows
- **Running:** `go run cmd/graft/main.go`
- **Testing:** `go test ./...`
  - Integration tests live in `internal/integration`.
- **Configuration:** Check `configs/example.env`. Environment variables drive `internal/config`.
- **Docker:** `deployments/docker-compose.yml` mounts data to `/data`.

## Integration Points
- **Webhooks:** Generic receiver at `func (h *WebhookHandler) ServeHTTP`.
- **Transformers:** `internal/transformer` handles payload modification before forwarding.

