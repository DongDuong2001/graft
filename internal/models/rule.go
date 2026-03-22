package models

import "fmt"

var (
	ErrRuleNotFound = fmt.Errorf("rule not found")
	ErrUnauthorized = fmt.Errorf("unauthorized")
)

// Rule defines how an incoming webhook should be processed and forwarded.
type Rule struct {
	ID                       string            `json:"id"`
	Name                     string            `json:"name"`
	Description              string            `json:"description"`
	ListenPath               string            `json:"listen_path"` // e.g. "/hook/github"
	RequiredSignature        bool              `json:"required_signature"`
	SignatureHeader          string            `json:"signature_header"`           // e.g. "X-Hub-Signature-256" or "Stripe-Signature"
	SignatureFormat          string            `json:"signature_format"`           // "hex" (GitHub-style) or "stripe_v1"
	SignatureTimestampHeader string            `json:"signature_timestamp_header"` // optional Unix seconds; mitigates replays for hex HMAC
	SignatureMaxSkewSeconds  int               `json:"signature_max_skew_seconds"` // default 300 when timestamp header is set
	SignatureSecret          string            `json:"-"`                          // encrypted at rest
	TransformTemplate        string            `json:"transform_template"`
	DestinationURL           string            `json:"destination_url"`
	DestinationMethod        string            `json:"destination_method"`
	DestinationHeaders       map[string]string `json:"destination_headers"` // plaintext in memory; encrypted at rest in DB
}

// Delivery records one forward attempt outcome for a rule.
type Delivery struct {
	ID         string `json:"id"`
	RuleID     string `json:"rule_id"`
	CreatedAt  int64  `json:"created_at"` // Unix seconds
	Success    bool   `json:"success"`
	StatusCode int    `json:"status_code"`
	ErrorMsg   string `json:"error_message,omitempty"`
	DurationMS int64  `json:"duration_ms"`
	RetryCount int    `json:"retry_count"`
}
