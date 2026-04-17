package models

import "fmt"

var (
	ErrRuleNotFound = fmt.Errorf("rule not found")
	ErrUnauthorized = fmt.Errorf("unauthorized")
)

// ---------------------------------------------------------------------------
// Rule defines how an incoming webhook should be processed and forwarded.
// It supports legacy single-destination mode as well as multi-destination
// fan-out, conditional routing, and pipeline transformations.
// ---------------------------------------------------------------------------
type Rule struct {
	ID                       string            `json:"id"`
	Name                     string            `json:"name"`
	Description              string            `json:"description"`
	ListenPath               string            `json:"listen_path"` // e.g. "/hook/github"
	RequiredSignature        bool              `json:"required_signature"`
	SignatureHeader          string            `json:"signature_header"`           // e.g. "X-Hub-Signature-256" or "Stripe-Signature"
	SignatureFormat          string            `json:"signature_format"`           // "hex", "stripe_v1", "shopify_hmac", "slack_v0", "twilio", "pagerduty"
	SignatureTimestampHeader string            `json:"signature_timestamp_header"` // optional Unix seconds; mitigates replays for hex HMAC
	SignatureMaxSkewSeconds  int               `json:"signature_max_skew_seconds"` // default 300 when timestamp header is set
	SignatureSecret          string            `json:"-"`                          // encrypted at rest
	TransformTemplate        string            `json:"transform_template"`         // legacy single-step Go template
	DestinationURL           string            `json:"destination_url"`            // legacy single destination
	DestinationMethod        string            `json:"destination_method"`         // legacy single destination
	DestinationHeaders       map[string]string `json:"destination_headers"`        // plaintext in memory; encrypted at rest in DB

	// --- Fan-out: multiple destinations per rule ---
	Destinations []Destination `json:"destinations,omitempty"`

	// --- Conditional routing: payload-based expressions ---
	Conditions []Condition `json:"conditions,omitempty"`

	// --- Pipeline: multi-step transformations ---
	TransformSteps []TransformStep `json:"transform_steps,omitempty"`

	// --- Per-rule rate limiting ---
	RateLimitMax    int    `json:"rate_limit_max,omitempty"`    // max requests per window (0 = unlimited)
	RateLimitWindow string `json:"rate_limit_window,omitempty"` // e.g. "1m", "10s"

	// --- Per-rule IP allowlist (CIDR notation) ---
	IPAllowlist []string `json:"ip_allowlist,omitempty"` // e.g. ["192.30.252.0/22", "185.199.108.0/22"]

	// --- Optional webhook endpoint API key (for rules without signature verification) ---
	RequireAPIKey bool   `json:"require_api_key,omitempty"` // enable API key auth
	APIKeyHeader  string `json:"api_key_header,omitempty"`  // default: "X-API-Key"
	APIKey        string `json:"-"`                         // encrypted at rest; simple shared secret for the endpoint
}

// ---------------------------------------------------------------------------
// Destination represents a single outbound target in a fan-out configuration.
// When Type is empty or "http", the standard HTTP forwarder is used.
// Other types ("slack", "discord", "teams", "email") use native connectors.
// ---------------------------------------------------------------------------
type Destination struct {
	URL       string            `json:"url"`
	Method    string            `json:"method,omitempty"`    // default POST
	Headers   map[string]string `json:"headers,omitempty"`   // per-destination headers
	Type      string            `json:"type,omitempty"`      // "http" (default), "slack", "discord", "teams", "email"
	Condition string            `json:"condition,omitempty"` // optional JSONPath or expression to gate this destination
}

// ---------------------------------------------------------------------------
// Condition defines a payload-based routing expression.
// If Field matches Value, the webhook is processed; otherwise it is skipped.
// Operator can be "eq", "neq", "contains", "exists".
// ---------------------------------------------------------------------------
type Condition struct {
	Field    string `json:"field"`           // JSONPath-like field (e.g. "action", "event.type")
	Operator string `json:"operator"`        // "eq", "neq", "contains", "exists"
	Value    string `json:"value,omitempty"` // comparison value (unused for "exists")
}

// ---------------------------------------------------------------------------
// TransformStep defines one stage in a multi-step transformation pipeline.
// Steps execute sequentially: output of step N becomes input of step N+1.
// ---------------------------------------------------------------------------
type TransformStep struct {
	Type   string `json:"type"`   // "go_template" or "javascript"
	Script string `json:"script"` // the template string or JS code
}

// ---------------------------------------------------------------------------
// Delivery records one forward attempt outcome for a rule.
// ---------------------------------------------------------------------------
type Delivery struct {
	ID         string `json:"id"`
	RuleID     string `json:"rule_id"`
	CreatedAt  int64  `json:"created_at"` // Unix seconds
	Success    bool   `json:"success"`
	Status     string `json:"status"` // "pending", "processing", "delivered", "failed", "dead_letter"
	StatusCode int    `json:"status_code"`
	ErrorMsg   string `json:"error_message,omitempty"`
	DurationMS int64  `json:"duration_ms"`
	RetryCount int    `json:"retry_count"`

	// --- Batch 2: Replay Support ---
	RequestBody []byte `json:"request_body,omitempty"` // The original incoming JSON payload
	RequestPath string `json:"request_path,omitempty"` // The path the webhook was received on
}

// --- Delivery status constants ---
const (
	StatusPending    = "pending"
	StatusProcessing = "processing"
	StatusDelivered  = "delivered"
	StatusFailed     = "failed"
	StatusDeadLetter = "dead_letter"
)

// ---------------------------------------------------------------------------
// EffectiveDestinations returns the destinations to forward to.
// If the new Destinations slice is populated, it is used (fan-out mode).
// Otherwise, the legacy DestinationURL/DestinationMethod/DestinationHeaders
// fields are wrapped into a single-element slice for backward compatibility.
// ---------------------------------------------------------------------------
func (r *Rule) EffectiveDestinations() []Destination {
	if len(r.Destinations) > 0 {
		return r.Destinations
	}
	// --- Backward compatibility: wrap legacy single destination ---
	if r.DestinationURL != "" {
		return []Destination{{
			URL:     r.DestinationURL,
			Method:  r.DestinationMethod,
			Headers: r.DestinationHeaders,
			Type:    "http",
		}}
	}
	return nil
}
