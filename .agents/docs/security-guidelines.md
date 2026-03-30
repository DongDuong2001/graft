# Security Guidelines for AI Agents

Operational quality and test expectations for agents working in this repo: **`.agents/SKILL.md`** and **`.agents/FEATURE-TEST-MATRIX.md`**.

All AI agents working on this project MUST strictly adhere to these security requirements. This Webhook-to-Anything bridge is intended to be self-hosted by small organizations and indie developers on the public internet.

1.  **Authentication & Authorization**
    *   All administrative endpoints (for creating/editing Webhook rules) MUST be protected by strong authentication (e.g., API keys, Bearer tokens).
    *   Requests without valid authentication MUST immediately return `401 Unauthorized`.

2.  **Webhook Validation & Security**
    *   Incoming webhooks often come from third parties (Stripe, GitHub, etc.). The agent MUST implement signature verification (e.g., HMAC-SHA256 checking against `X-Hub-Signature-256` or equivalent).
    *   Replay attack prevention MUST be considered (e.g., timestamp checking in signatures if supported by the provider).

3.  **Data Security & Privacy (At Rest and In Transit)**
    *   Any secret keys (API keys for destination APIs, Webhook shared secrets) stored in the database (e.g., SQLite) MUST be encrypted at rest using strong encryption like AES-GCM.
    *   Do NOT log raw request payloads or secrets in the application logs unless heavily redacted.
    *   Enforce HTTPS/TLS for all external communication.

4.  **Resilience & Denial of Service Protection**
    *   Implement rate-limiting (e.g., Token Bucket) on both the receiver (inbound) and forwarder (outbound).
    *   Set stringent connection timeouts (`ReadTimeout`, `WriteTimeout`, `IdleTimeout`) on the Go HTTP Server to prevent Slowloris attacks.
    *   Limit the maximum request body size (e.g., `http.MaxBytesReader`) to prevent memory exhaustion attacks.

5.  **Input Sanitization and Validation**
    *   Never trust JSON inputs. Unmarshal strictly and validate structure before processing.
    *   When using low-code template engines for transformations (like `text/template` in Go), ensure they cannot execute arbitrary shell commands or read arbitrary files. Ensure templates are isolated.

6.  **Error Handling**
    *   Do NOT leak internal architecture or stack traces in HTTP responses to clients. Log internally, return generic error messages externally (e.g., `500 Internal Server Error`).

## Strict Enforcement
You must review changes against these rules before completing any task. Violation of these rules is a critical failure.
