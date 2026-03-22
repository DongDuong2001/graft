# Feature → test matrix

Every **feature area** below must have automated tests before the project is treated as **ready for public** use. When you add or change behavior in a row, update or add tests in the listed files.

| Feature area | Package | Test file(s) | Notes |
|--------------|---------|----------------|-------|
| Environment config | `internal/config` | `config_test.go` | Required keys, defaults |
| Composition / wiring | `internal/app` | _(integration covers)_ | `app.Run` is thin; e2e in `internal/integration` |
| HTTP server wrapper | `internal/server` | `server_test.go` | Addr and timeouts from config |
| Route tree + middleware glue | `internal/router` | `router_test.go` | Health, mount shape |
| Admin REST API | `internal/httpapi` | `admin_test.go` | **Requires CGO** for SQLite |
| Webhook ingress + verify | `internal/httpapi` | `webhook_test.go`, `signature_test.go` | CGO for handler tests |
| JSON → template → JSON | `internal/transformer` | `transformer_test.go` | Success, passthrough, invalid JSON |
| Outbound HTTP (retries) | `internal/connectors` | `http_test.go` | httptest; observability delta |
| Counters / admin metrics JSON | `internal/observability` | `metrics_test.go` | Snapshot, `WriteMetricsJSON` |
| AES-GCM at rest | `internal/crypto` | `crypto_test.go` | |
| SQLite rules & deliveries | `internal/storage` | `sqlite_test.go` | **Requires CGO** |
| Auth + rate limit + IP | `internal/middleware` | `*_test.go` | |
| Domain models | `internal/models` | `rule_test.go` | JSON safety |
| Shared SQLite test helper | `internal/testutil` | _(no tests)_ | |
| E2E admin → webhook → destination | `internal/integration` | `graft_test.go` | **Requires CGO** |
| Process entry | `cmd/graft` | _(none)_ | Thin `main`; covered by integration |

## Packages without dedicated `_test.go`

- **`cmd/graft`** — Entrypoint only; integration tests exercise real wiring.
- **`internal/app`** — Orchestration only; same as above.

## CI recommendation

- Job **unit** (any OS): `go test ./...` — storage/httpapi tests may skip without CGO.
- Job **full** (Linux + `gcc`): `CGO_ENABLED=1 go test -race ./...` — required before a public release.
