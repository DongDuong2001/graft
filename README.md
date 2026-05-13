[English](README.md) | [Tiếng Việt](README-vi.md)

# Graft: Self-Hosted Webhook Bridge

<a href="https://forg.to/products/graft" target="_blank" rel="noopener">
  <img src="https://forg.to/api/badges/upvote/graft?theme=dark&shape=square" alt="Graft - Upvote on Forg on forg." height="40" />
</a>
<a href="https://unikorn.vn/p/graft?ref=embed-graft" target="_blank">
  <img src="https://unikorn.vn/api/widgets/badge/graft?theme=light" alt="Graft trên Unikorn.vn" height="40" />
</a>

<a href="https://github.com/DongDuong2001/graft/actions/workflows/ci.yml" target="_blank">
  <img src="https://github.com/DongDuong2001/graft/actions/workflows/ci.yml/badge.svg" alt="CI" height="40" />
</a>

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

- `make setup`: Generate `.env` from the example and create random keys.
- `make build`: Build the binary to `bin/graft`.
- `make run`: Run the application.
- `make test`: Run all tests.
- `make fmt`: Format code.
- `make lint`: Run golangci-lint.
- `make vet`: Run go vet.
- `make docker-build`: Build the Docker image.
- `make docker-up`: Start containers in detached mode.
- `make docker-down`: Stop containers.
- `make docker-logs`: Follow Docker logs.
- `make clean`: Clean build artifacts.

### Graft CLI Tools

Graft has a built-in CLI powered by [Cobra](https://github.com/spf13/cobra). Running `graft` without arguments runs the webhook bridge, but it also has other commands:

- `graft start`: (Default) Starts the webhook bridge.
- `graft version`: Prints the current version, commit hash, and build date.
- `graft docs <output-dir>`: Autogenerates markdown documentation for the CLI in the specified directory.
- `graft completion <shell>`: Generates auto-completion scripts for your shell.

*(Note: If you build with `make`, version details are automatically injected into the binary into the `version` command)*

## Cross-Platform Compilation

Need to build Graft for a different OS? Use the `build-all` target:
```bash
make build-all
```
This builds standard binaries for Linux, macOS, and Windows (amd64/arm64) and places them in the `bin/` directory.

### Manual Steps

1.  **Clone the repository:**
    ```bash
    git clone https://github.com/your-org/graft.git
    cd graft
    ```

2.  **Set up environment:**
    If you're using `make`, just run `make setup`.
    Otherwise, copy `configs/example.env` to `.env` and fill in the values randomly.
    ```bash
    cp configs/example.env .env
    ```
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

## AI-Assisted Workflow

Graft uses the PUDO workflow for meaningful AI-assisted changes:

1. **Plan** - define scope, constraints, success criteria, and risks.
2. **Understand** - inspect the affected code, tests, and security surfaces before editing.
3. **Develop** - implement in small steps and update tests with behavior changes.
4. **Optimize** - self-review, verify, document, and summarize the final change.

See [`AGENTS.md`](AGENTS.md), [`.agents/SKILL.md`](.agents/SKILL.md), and [`.agents/workflows/pudo.md`](.agents/workflows/pudo.md) for the project-specific instructions.


## License

[MIT License](LICENSE)


## Contributing

Please see our strict [Contributing Guidelines](CONTRIBUTING.md) for details on our code of conduct, branching strategy, commit conventions, and the process for submitting pull requests to us.
