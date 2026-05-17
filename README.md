[English](README.md) | [Tiếng Việt](README-vi.md)

# Graft: Self-Hosted Webhook Bridge

<a href="https://forg.to/products/graft" target="_blank" rel="noopener">
  <img src="https://forg.to/api/badges/upvote/graft?theme=dark&shape=square" alt="Graft - Upvote on Forg" height="40" />
</a>
<a href="https://unikorn.vn/p/graft?ref=embed-graft" target="_blank" rel="noopener">
  <img src="https://unikorn.vn/api/widgets/badge/graft?theme=light" alt="Graft on Unikorn.vn" height="40" />
</a>
<a href="https://github.com/DongDuong2001/graft/actions/workflows/ci.yml" target="_blank" rel="noopener">
  <img src="https://github.com/DongDuong2001/graft/actions/workflows/ci.yml/badge.svg" alt="CI" height="40" />
</a>

Graft is a lightweight webhook-to-anything bridge written in Go. It receives incoming webhooks on `/hook/{path}`, optionally verifies signatures, can transform JSON, and forwards the result to one or more destinations. It is designed to be self-hosted and run in Docker or Kubernetes.

## Quick Start (Docker)

1. Create `.env` from the example and fill in values:

```bash
cp configs/example.env .env
```

2. Start the service:

```bash
docker compose -f deployments/docker-compose.yml up -d --build
```

3. Verify:

```bash
curl http://localhost:8080/healthz
```

## Features

- Ingress: accepts POST webhooks on `/hook/{path}`
- Routing: matches requests based on URL path
- Security: signature verification, admin API key auth, AES-GCM encryption at rest
- Transformation: `text/template` and pipeline steps
- Resilience: configurable retries and timeouts for outbound requests
- Observability: metrics endpoint (`/metrics`) and structured JSON logs
- Storage: SQLite-backed rules and delivery history

## Prerequisites

- Go 1.25+ (local development)
- Docker with Compose v2 (`docker compose`) (deployment)
- Optional: OpenSSL (key generation), Make (convenience)

## Configuration

Graft is configured via environment variables. See `configs/example.env`.

| Variable | Description | Required | Default |
| :--- | :--- | :--- | :--- |
| `MASTER_KEY` | 32-byte hex string (64 hex chars) for encryption at rest. | Yes | - |
| `ADMIN_API_KEY` | Secret key for Admin API access. | Yes | - |
| `DB_PATH` | Path to the SQLite database file. | No | `./rules.db` |
| `PORT` | HTTP port to listen on. | No | `8080` |
| `FORWARD_TIMEOUT` | Outbound request timeout. | No | `30s` |
| `FORWARD_MAX_RETRIES` | Outbound retry count. | No | `3` |
| `FORWARD_RETRY_BASE` | Base backoff for retries. | No | `200ms` |

Generate a master key:

```bash
openssl rand -hex 32
```

## Running Locally

SQLite uses `github.com/mattn/go-sqlite3` and requires CGO. If storage/httpapi/integration tests are skipped on your machine, run with a working C toolchain and `CGO_ENABLED=1`.

```bash
cp configs/example.env .env
go run cmd/graft/main.go
```

## API Usage

All Admin API endpoints are under `/api/v1/` and require:

```
Authorization: Bearer <ADMIN_API_KEY>
```

Create a rule:

```bash
curl -X POST http://localhost:8080/api/v1/rules \
  -H "Authorization: Bearer YOUR_ADMIN_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "GitHub Push to Slack",
    "listen_path": "/hook/github-push",
    "required_signature": true,
    "signature_header": "X-Hub-Signature-256",
    "signature_format": "hex",
    "signature_secret": "your-github-webhook-secret",
    "destination_url": "https://hooks.slack.com/services/...",
    "destination_method": "POST"
  }'
```

List rules:

```bash
curl http://localhost:8080/api/v1/rules \
  -H "Authorization: Bearer YOUR_ADMIN_KEY"
```

Send a webhook:

```bash
curl -X POST http://localhost:8080/hook/github-push \
  -H "Content-Type: application/json" \
  -H "X-Hub-Signature-256: sha256=generated-signature..." \
  -d '{"ref":"refs/heads/main"}'
```

## Testing

```bash
go test ./...
go test -v ./...
```

## AI-Assisted Workflow

This repo uses the PUDO workflow (Plan, Understand, Develop, Optimize). See `AGENTS.md` and `.agents/workflows/pudo.md`.

## License

[MIT License](LICENSE)

## Contributing

See `CONTRIBUTING.md`.
