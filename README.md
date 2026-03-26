# Graft: Self-Hosted Webhook Bridge
<a href="https://forg.to/products/graft" target="_blank" rel="noopener">
  <img src="https://forg.to/api/badges/upvote/graft?theme=dark&shape=square" alt="Graft - Upvote on Forg on forg." height="48" />
</a>

[![CI](https://github.com/DongDuong2001/graft/actions/workflows/ci.yml/badge.svg)](https://github.com/DongDuong2001/graft/actions/workflows/ci.yml)

Graft is a lightweight, secure webhook-to-anything bridge written in Go. It receives incoming webhooks, validates signatures (from providers like GitHub or Stripe), optionally transforms the payload using templates, and forwards the result to another destination. It is designed to be self-hosted and run in Docker or Kubernetes.

## Features

- **Ingress**: Accepts POST webhooks on `/hook/{path}`.
- **Routing**: Matches requests based on the URL path.
- **Security**:
  - Validates webhook signatures (HMAC-SHA256, Stripe v1).
  - Admin API secured by an API key.
  - Secrets (like webhook signing keys) are encrypted at rest using AES-GCM.
- **Transformation**: Supports `text/template` for payload modification before forwarding.
- **Resilience**: Configurable retries and timeouts for outbound requests.
- **Observability**: Prometheus-ready metrics endpoint (`/metrics`) and JSON structured logs.
- **Storage**: SQLite-backed persistence for rules and delivery history.

## Prerequisites

- **Go 1.26+** (for local development)
- **Docker** & **Docker Compose** (recommended for deployment)
- **OpenSSL** (optional, for generating keys)
- **Make** (optional, for easier commands)

### Installing Make on Windows

If you don't have `make` installed, you can install it via Chocolatey or Scoop:

**Chocolatey:**
```powershell
choco install make
```

**Scoop:**
```powershell
scoop install make
```

Alternatively, you can run the `go` commands directly as described below.

## Configuration

Graft is configured via environment variables. See [`configs/example.env`](configs/example.env) for a template.

### Key Variables

| Variable | Description | Required | Default |
| :--- | :--- | :--- | :--- |
| `MASTER_KEY` | 32-byte hex string for encryption at rest. | **Yes** | - |
| `ADMIN_API_KEY` | Secret key for accessing the Admin API. | **Yes** | - |
| `DB_PATH` | Path to the SQLite database file. | No | `./rules.db` |
| `PORT` | HTTP port to listen on. | No | `8080` |

### Generating a Master Key

```bash
openssl rand -hex 32
```

## Running Locally

### Using Make

If you have `make` installed, you can use the provided `Makefile` for common tasks:

- `make build`: Build the binary to `bin/graft`.
- `make run`: Run the application.
- `make test`: Run all tests.
- `make vet`: Run go vet.
- `make docker-build`: Build the Docker image.
- `make clean`: Clean build artifacts.

### Manual Steps

1.  **Clone the repository:**
    ```bash
    git clone https://github.com/your-org/graft.git
    cd graft
    ```

2.  **Set up environment:**
    Copy `configs/example.env` to `.env` and fill in the values.
    ```bash
    cp configs/example.env .env
    # Edit .env to set MASTER_KEY and ADMIN_API_KEY
    ```

3.  **Run the application:**
    ```bash
    # Load env vars (e.g., using export or a tool like dotenv)
    # On Linux/Mac:
    export $(grep -v '^#' .env | xargs)
    go run cmd/graft/main.go
    ```
    
    *Note: Ensure `CGO_ENABLED=1` is set if running on an environment where it's not default, as SQLite requires CGO.*

4.  **Verify it's running:**
    ```bash
    curl http://localhost:8080/healthz
    # Output: {"status":"ok"}
    ```

## Running with Docker

Use the provided `docker-compose.yml` to spin up Graft quickly.

1.  **Configure `.env`:**
    Ensure `configs/example.env` (or your `.env` file referenced in compose) has valid keys.

2.  **Start the service:**
    ```bash
    docker-compose -f deployments/docker-compose.yml up -d --build
    ```

    The service will be available at `http://localhost:8080`. Data will be persisted in the `graft-data` volume.

## API Usage

### Admin API

All admin endpoints are under `/api/v1/` and require the `Authorization: Bearer <ADMIN_API_KEY>` header.

#### Create a Rule

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

#### List Rules

```bash
curl http://localhost:8080/api/v1/rules -H "Authorization: Bearer YOUR_ADMIN_KEY"
```

### Sending Webhooks

Once a rule is created, you can send webhooks to the configured `listen_path`.

```bash
# Example for the rule above
curl -X POST http://localhost:8080/hook/github-push \
  -H "Content-Type: application/json" \
  -H "X-Hub-Signature-256: sha256=generated-signature..." \
  -d '{"ref": "refs/heads/main", ...}'
```

## Testing

Run the test suite using `go test`. Note that integration tests require CGO enabled for SQLite.

```bash
go test ./...
# For verbose output
go test -v ./...
```

To run only unit tests (skipping tests that might require external setups or heavy DB):
```bash
go test -short ./...
```

## License

[MIT License](LICENSE)

## Contributing

### Git Commit Convention

We follow the [Conventional Commits](https://www.conventionalcommits.org/) specification for commit messages. This helps in generating changelogs and versioning.

**Structure:**
```
<type>(<scope>): <subject>
```

**Types:**
- `feat`: A new feature
- `fix`: A bug fix
- `docs`: Documentation only changes
- `style`: Changes that do not affect the meaning of the code (white-space, formatting, missing semi-colons, etc)
- `refactor`: A code change that neither fixes a bug nor adds a feature
- `perf`: A code change that improves performance
- `test`: Adding missing tests or correcting existing tests
- `chore`: Changes to the build process or auxiliary tools and libraries such as documentation generation

**Example:**
```
feat(auth): add JWT support for administrative API
fix(engine): resolve null pointer exception when payload is empty
docs: update README with deployment instructions
```

### Pull Requests

Ensure all checks pass before submitting a PR:
1. Run tests: `make test`
2. Run linter: `make vet`
3. Check formatting: `go fmt ./...`
