# Graft Project - Claude Agent Guide

## Default Workflow: PUDO

Follow **PUDO: Plan -> Understand -> Develop -> Optimize** for meaningful tasks in this repository.

1. **Plan** - clarify scope, success criteria, constraints, risks, and the implementation path before writing code.
2. **Understand** - read the affected code and tests first; match Graft's existing patterns for handlers, services, storage, models, config, and security.
3. **Develop** - implement step by step, keep a checklist current, and add or update tests with behavior changes.
4. **Optimize** - self-review, run verification, update docs when needed, and finish with a concise walkthrough.

Tiny fixes can use a compressed cycle, but do not skip reading the touched code. If implementation reveals a bad assumption, loop back to Plan or Understand. See `.agents/workflows/pudo.md` for the project-specific workflow.

## Project Overview

**Graft** is a lightweight, self-hosted webhook-to-anything bridge written in Go. It receives incoming webhooks, validates signatures (HMAC-SHA256, Stripe v1), optionally transforms payloads using Go templates or JavaScript, and forwards them to configured destinations.

**Key Characteristics:**
- Language: Go 1.25+
- Database: SQLite (with CGO)
- Architecture: Queue-based async processing with worker pools
- Security: AES-GCM encryption, signature verification, rate limiting, brute force protection
- Deployment: Docker/Kubernetes ready

## Project Structure

```
D:\Graft/
├── cmd/graft/main.go              # Entry point
├── internal/
│   ├── app/app.go                 # Application wiring and startup
│   ├── audit/audit.go             # Security audit logging
│   ├── config/config.go           # Environment configuration
│   ├── server/server.go           # HTTP server with TLS support
│   ├── server/tls.go              # TLS certificate management
│   ├── router/router.go           # Route definitions with security middleware
│   ├── middleware/
│   │   ├── auth.go                # Admin API key authentication
│   │   ├── bruteforce.go          # Brute force protection
│   │   ├── cidr.go                # CIDR-based IP allowlisting
│   │   ├── cors.go                # CORS middleware
│   │   ├── ratelimit.go           # Token bucket rate limiter
│   │   ├── security.go            # Security headers middleware
│   │   ├── webhookauth.go         # Webhook endpoint API keys
│   │   └── middleware.go          # Logging & IP extraction
│   ├── engine/
│   │   ├── engine.go              # Main processing pipeline
│   │   ├── worker.go              # Background worker pool
│   │   └── conditions.go          # Condition evaluation
│   ├── webhook/webhook.go         # Webhook model & signature verification
│   ├── models/rule.go             # Rule & Delivery data models
│   ├── storage/sqlite.go          # SQLite repository implementation
│   ├── crypto/crypto.go           # AES-GCM encryption/decryption
│   ├── transformer/               # Payload transformation
│   ├── connectors/                # Destination connectors (HTTP, Slack, Discord, etc.)
│   └── ui/ui.go                   # Embedded web UI handler
├── configs/example.env            # Configuration template
├── deployments/                   # Docker & K8s manifests
└── SECURITY.md                    # Security hardening guide
```

## Coding Conventions

### Go Style
- Use standard Go formatting (`go fmt`)
- Follow Go naming conventions: CamelCase for exported, camelCase for unexported
- Prefer explicit error handling over panics
- Use `log/slog` for structured logging
- Prefer `fmt.Errorf` with `%w` for error wrapping

### Project-Specific Patterns

**Middleware Pattern:**
```go
func SomeMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Pre-processing
        next.ServeHTTP(w, r)
        // Post-processing (if needed)
    })
}
```

**Configuration Pattern:**
```go
// In config/config.go
func Load() (Config, error) {
    c := Config{
        Field: envOr("ENV_VAR", "default"),
    }
    // Validation
    if c.RequiredField == "" {
        return Config{}, fmt.Errorf("REQUIRED_FIELD is required")
    }
    return c, nil
}
```

**Handler Pattern:**
```go
type SomeHandler struct {
    deps *Dependencies
}

func NewSomeHandler(deps *Dependencies) *SomeHandler {
    return &SomeHandler{deps: deps}
}

func (h *SomeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    // Implementation
}
```

### Security Conventions

**Always use constant-time comparison for secrets:**
```go
import "crypto/subtle"
if subtle.ConstantTimeCompare([]byte(expected), []byte(provided)) != 1 {
    // Unauthorized
}
```

**Encrypt secrets at rest:**
```go
// In crypto/crypto.go
encrypted, err := crypto.Encrypt(plaintext, masterKey)
decrypted, err := crypto.Decrypt(encrypted, masterKey)
```

## Key Components Reference

### Middleware Stack (in order)

1. **Security Headers** (`middleware/security.go`)
   - CSP, HSTS, X-Frame-Options, etc.
   - Dev mode available via `DEV_MODE=1`

2. **CORS** (`middleware/cors.go`)
   - Configurable via `CORS_ORIGINS`

3. **Rate Limiting** (`middleware/tokenbucket.go`)
   - Token bucket algorithm
   - Per-IP tracking

4. **Brute Force Protection** (`middleware/bruteforce.go`)
   - Exponential backoff
   - Tracks by IP

5. **Authentication** (`middleware/auth.go`)
   - Admin API key validation

### Rule Model (`models/rule.go`)

Key fields:
- `ListenPath`: Webhook endpoint path
- `RequiredSignature`: Enable HMAC verification
- `SignatureFormat`: "hex" (GitHub) or "stripe_v1"
- `IPAllowlist`: CIDR ranges for source filtering
- `RequireAPIKey`: Enable simple API key auth
- `TransformSteps`: Multi-step transformations
- `Destinations`: Fan-out destinations

### Testing

**Run all tests:**
```bash
go test ./...
```

**Run with CGO (required for SQLite):**
```bash
CGO_ENABLED=1 go test ./...
```

**Test structure:**
- Unit tests: `*_test.go` alongside source
- Integration tests: `internal/integration/graft_test.go`
- Use `testutil` package for shared test helpers

### Environment Variables Reference

**Critical (Required):**
- `MASTER_KEY`: 64 hex characters (32 bytes) for AES-GCM
- `ADMIN_API_KEY`: API key for admin endpoints

**Security:**
- `TLS_ENABLED`: Enable TLS
- `TLS_AUTO_GENERATE`: Generate self-signed certs
- `TLS_CERT_FILE`/`TLS_KEY_FILE`: Custom certificates
- `CORS_ORIGINS`: Comma-separated allowed origins
- `RATE_LIMIT_BURST`: Token bucket capacity
- `RATE_LIMIT_REFILL_PER_SEC`: Sustained rate
- `BRUTE_FORCE_MAX_FAILURES`: Lockout threshold
- `AUDIT_ENABLED`: Enable audit logging

**Development:**
- `DEV_MODE=1`: Relaxed security headers, dev CORS

## Common Tasks

### Adding a New Middleware

1. Create file in `internal/middleware/{name}.go`
2. Implement `func(next http.Handler) http.Handler` pattern
3. Add to `router.BuildSecurityConfig()` or `router.NewRootMux()` as appropriate
4. Write tests in `internal/middleware/{name}_test.go`

### Adding a New Connector

1. Create file in `internal/connectors/{name}.go`
2. Implement `Connector` interface
3. Register in `internal/connectors/registry.go`

### Adding a Database Migration

1. Add migration SQL to `internal/storage/sqlite.go` in `runMigrations()`
2. Update `models/rule.go` if schema changes
3. Test with `internal/storage/sqlite_test.go`

### Security Checklist for New Features

- [ ] Input validation
- [ ] Rate limiting considered
- [ ] Authentication/authorization handled
- [ ] No hardcoded secrets
- [ ] Encryption at rest for sensitive data
- [ ] Audit logging for security events

## Security Considerations

**Never:**
- Log sensitive data (API keys, signatures)
- Use timing-attack-vulnerable string comparison
- Trust client IP without `TRUST_FORWARDED_HEADERS` consideration
- Disable TLS in production

**Always:**
- Use constant-time comparison for secrets
- Validate and sanitize inputs
- Return generic error messages to clients
- Log security events to audit log

## Build & Deploy

**Build:**
```bash
go build -o bin/graft ./cmd/graft
```

**With CGO (for SQLite):**
```bash
CGO_ENABLED=1 go build -o bin/graft ./cmd/graft
```

**Docker:**
```bash
docker-compose -f deployments/docker-compose.yml up -d --build
```

## Documentation

- `README.md`: General usage
- `SECURITY.md`: Security hardening guide
- `configs/example.env`: Configuration reference
