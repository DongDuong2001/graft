# Security Hardening Guide for Graft

This document describes the security features implemented in Graft to help indie developers and small organizations self-host securely.

## Quick Start for Production

```bash
# Generate strong keys
MASTER_KEY=$(openssl rand -hex 32)
ADMIN_API_KEY=$(openssl rand -base64 48)

# Minimal secure configuration
cat > .env << EOF
MASTER_KEY=$MASTER_KEY
ADMIN_API_KEY=$ADMIN_API_KEY
TLS_ENABLED=1
TLS_AUTO_GENERATE=1
RATE_LIMIT_BURST=20
RATE_LIMIT_REFILL_PER_SEC=10
BRUTE_FORCE_MAX_FAILURES=5
AUDIT_ENABLED=1
EOF
```

## Security Features

### 1. TLS/HTTPS Support

**Why:** Prevents eavesdropping and man-in-the-middle attacks.

**Configuration:**
```bash
# Option A: Auto-generate self-signed certificate
TLS_ENABLED=1
TLS_AUTO_GENERATE=1
TLS_AUTO_GENERATE_DIR=./certs

# Option B: Use your own certificate
TLS_ENABLED=1
TLS_CERT_FILE=/path/to/cert.pem
TLS_KEY_FILE=/path/to/key.pem
```

### 2. Security Headers

**Applied automatically:**
- `X-Content-Type-Options: nosniff` - Prevents MIME type sniffing
- `X-Frame-Options: DENY` - Prevents clickjacking
- `X-XSS-Protection: 1; mode=block` - Legacy XSS protection
- `Content-Security-Policy` - Restricts resource loading
- `Strict-Transport-Security` - HTTPS enforcement (when TLS enabled)
- `Referrer-Policy: strict-origin-when-cross-origin`
- `Permissions-Policy` - Restricts browser features

**Development mode** (`DEV_MODE=1`) uses relaxed CSP to allow CDN resources.

### 3. Token Bucket Rate Limiting

**Why:** Prevents abuse and resource exhaustion. More flexible than fixed-window.

**Configuration:**
```bash
# Allow bursts of 20, then sustained 10 requests/second
RATE_LIMIT_BURST=20
RATE_LIMIT_REFILL_PER_SEC=10
```

**Headers returned:**
- `X-RateLimit-Limit` - Maximum burst
- `X-RateLimit-Remaining` - Current tokens available
- `Retry-After` - Seconds to wait when rate limited

### 4. Brute Force Protection

**Why:** Prevents credential stuffing on admin endpoints.

**Configuration:**
```bash
BRUTE_FORCE_MAX_FAILURES=5        # Lock after 5 failures
BRUTE_FORCE_LOCKOUT_BASE=5m       # Initial 5-minute lockout
BRUTE_FORCE_MAX_LOCKOUT=1h        # Max 1 hour (exponential backoff)
```

**Behavior:**
- Tracks failed auth attempts by IP
- Exponential backoff: 5min → 10min → 20min → ... → 1hour
- Successful auth clears the failure count

### 5. CORS Protection

**Why:** Prevents cross-site request forgery.

**Configuration:**
```bash
# Production: specific origins
CORS_ORIGINS=https://admin.example.com,https://dashboard.example.com

# Development: allow localhost
CORS_ORIGINS=http://localhost:*,http://127.0.0.1:*

# Allow credentials (cookies/auth headers)
CORS_CREDENTIALS=1
```

### 6. CIDR IP Allowlisting

**Why:** Restrict webhook endpoints to specific networks (e.g., GitHub IPs).

**Per-rule configuration:**
```json
{
  "ip_allowlist": [
    "192.30.252.0/22",
    "185.199.108.0/22",
    "2620:112:3000::/44"
  ]
}
```

Supports IPv4, IPv6, and CIDR notation.

### 7. Webhook Endpoint API Keys

**Why:** Additional authentication layer when HMAC signatures aren't available.

**Per-rule configuration:**
```json
{
  "require_api_key": true,
  "api_key_header": "X-API-Key",
  "api_key": "your-secret-key"
}
```

### 8. Audit Logging

**Why:** Security event trail for forensics and compliance.

**Configuration:**
```bash
AUDIT_ENABLED=1
AUDIT_FILE_PATH=/var/log/graft/audit.log  # Optional, defaults to stdout
AUDIT_MIN_SEVERITY=info                   # info, warning, critical
```

**Events logged:**
- `auth.success` / `auth.failure` / `auth.lockout`
- `access.denied` / `ratelimit.hit`
- `webhook.received` / `webhook.delivered`
- `signature.invalid` / `replay.detected`
- `rule.created` / `rule.updated` / `rule.deleted`

### 9. Existing Signature Verification

**Supported formats:**
- `hex` - GitHub-style HMAC-SHA256 (`sha256=...`)
- `stripe_v1` - Stripe signature scheme with timestamp

**Timestamp replay protection:**
```json
{
  "signature_timestamp_header": "X-Webhook-Timestamp",
  "signature_max_skew_seconds": 300
}
```

## Security Checklist for Self-Hosting

### Deployment
- [ ] Use strong `MASTER_KEY` (64 hex chars)
- [ ] Use strong `ADMIN_API_KEY` (48+ chars)
- [ ] Enable TLS (`TLS_ENABLED=1`)
- [ ] Set `DEV_MODE=0` in production
- [ ] Configure rate limiting appropriately
- [ ] Enable audit logging

### Webhook Endpoints
- [ ] Enable signature verification when available
- [ ] Use IP allowlists for known sources
- [ ] Add endpoint API keys as fallback
- [ ] Set timestamp headers to prevent replays

### Admin Access
- [ ] Keep `ADMIN_API_KEY` secret
- [ ] Brute force protection is automatic
- [ ] Consider IP allowlisting admin endpoints (via reverse proxy)

### Database
- [ ] SQLite file permissions: `chmod 600 rules.db`
- [ ] Store on encrypted volume if possible
- [ ] Back up encrypted data regularly

## Architecture

```
Request → [Security Headers] → [CORS] → [Rate Limit] → [Auth] → Handler
                                         ↓
                                    [Brute Force]
                                    Protection
```

**Order matters:** Security headers are applied first so they're present even on error responses.

## Threat Model

### What Graft Protects Against
- ✓ Credential brute force (exponential backoff)
- ✓ Request flooding (token bucket rate limiting)
- ✓ Replay attacks (timestamp verification)
- ✓ Clickjacking (frame options)
- ✓ XSS (CSP + XSS protection header)
- ✓ MITM (TLS)
- ✓ Timing attacks (constant-time comparison)

### What You Still Need
- Network-level protection (firewall, DDoS mitigation)
- Host security (updates, intrusion detection)
- Backup encryption
- Log retention and monitoring

## Migration from Previous Versions

1. Add new environment variables to `.env`
2. Regenerate `MASTER_KEY` if it wasn't 64 hex chars
3. Review webhook rules and add `ip_allowlist` where appropriate
4. Enable audit logging for visibility

## Reporting Security Issues

If you discover a security vulnerability, please report it responsibly.
