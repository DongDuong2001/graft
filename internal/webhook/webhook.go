package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/DongDuong2001/graft/internal/models"
)

// Webhook represents an incoming webhook request.
type Webhook struct {
	ID         string
	Path       string
	Method     string
	Headers    http.Header
	Body       []byte
	ReceivedAt time.Time
}

// NewFromRequest creates a Webhook from an HTTP request.
// It reads the body, so the caller should handle any errors or limits before calling this if needed.
// However, typically this function consumes the body.
func NewFromRequest(r *http.Request, body []byte) *Webhook {
	return &Webhook{
		Path:       r.URL.Path,
		Method:     r.Method,
		Headers:    r.Header,
		Body:       body,
		ReceivedAt: time.Now(),
	}
}

// VerifySignature checks if the webhook payload matches the expected signature defined in the rule.
func (w *Webhook) VerifySignature(rule *models.Rule, secret string) error {
	if !rule.RequiredSignature {
		return nil
	}

	if rule.SignatureHeader == "" {
		return fmt.Errorf("rule requires signature but no header defined")
	}

	headerValue := w.Headers.Get(rule.SignatureHeader)
	if headerValue == "" {
		return fmt.Errorf("missing signature header %q", rule.SignatureHeader)
	}

	// Verify timestamp if required
	if rule.SignatureTimestampHeader != "" {
		tsValue := w.Headers.Get(rule.SignatureTimestampHeader)
		if err := verifyTimestampSkew(rule.SignatureTimestampHeader, tsValue, rule.SignatureMaxSkewSeconds); err != nil {
			return err
		}
	}

	switch rule.SignatureFormat {
	case "stripe_v1":
		if err := verifyStripeReplay(headerValue, rule.SignatureMaxSkewSeconds); err != nil && rule.SignatureTimestampHeader == "" {
			// Stripe header embeds timestamp, so we check it if specific timestamp header wasn't checked above
			return err
		}
		if !verifyStripeV1(w.Body, secret, headerValue) {
			return fmt.Errorf("invalid stripe signature")
		}
	case "hex", "":
		// Default to hex
		if !verifySignatureHex(w.Body, secret, headerValue) {
			return fmt.Errorf("invalid hex signature")
		}
	default:
		return fmt.Errorf("unknown signature format %q", rule.SignatureFormat)
	}

	return nil
}

// verifySignatureHex checks HMAC-SHA256 in hex format (e.g. GitHub).
func verifySignatureHex(payload []byte, secret, headerValue string) bool {
	if strings.HasPrefix(headerValue, "sha256=") {
		headerValue = strings.TrimPrefix(headerValue, "sha256=")
	}

	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(payload)
	expectedMAC := hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(expectedMAC), []byte(headerValue))
}

// verifyStripeV1 checks Stripe-style signatures (t=timestamp,v1=signature).
func verifyStripeV1(payload []byte, secret, headerValue string) bool {
	var ts string
	var sigs []string
	for _, part := range strings.Split(headerValue, ",") {
		part = strings.TrimSpace(part)
		switch {
		case strings.HasPrefix(part, "t="):
			ts = strings.TrimPrefix(part, "t=")
		case strings.HasPrefix(part, "v1="):
			sigs = append(sigs, strings.TrimPrefix(part, "v1="))
		}
	}
	if ts == "" || len(sigs) == 0 {
		return false
	}

	signed := ts + "." + string(payload)
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(signed))
	expected := hex.EncodeToString(mac.Sum(nil))

	for _, s := range sigs {
		if hmac.Equal([]byte(expected), []byte(s)) {
			return true
		}
	}
	return false
}

func verifyTimestampSkew(headerName, headerValue string, maxSkew int) error {
	if headerName == "" {
		return nil
	}
	raw := strings.TrimSpace(headerValue)
	if raw == "" {
		return fmt.Errorf("missing timestamp header %q", headerName)
	}
	sec, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid timestamp in %q", headerName)
	}
	if maxSkew <= 0 {
		maxSkew = 300
	}
	event := time.Unix(sec, 0)
	delta := time.Since(event)
	if delta < 0 {
		delta = -delta
	}
	if delta > time.Duration(maxSkew)*time.Second {
		return fmt.Errorf("timestamp outside allowed skew")
	}
	return nil
}

func verifyStripeReplay(headerValue string, maxSkew int) error {
	ts, ok := parseStripeTimestamp(headerValue)
	if !ok {
		return fmt.Errorf("stripe signature missing timestamp")
	}
	if maxSkew <= 0 {
		maxSkew = 300
	}
	event := time.Unix(ts, 0)
	delta := time.Since(event)
	if delta < 0 {
		delta = -delta
	}
	if delta > time.Duration(maxSkew)*time.Second {
		return fmt.Errorf("timestamp outside allowed skew")
	}
	return nil
}

func parseStripeTimestamp(headerValue string) (int64, bool) {
	for _, part := range strings.Split(headerValue, ",") {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "t=") {
			v := strings.TrimPrefix(part, "t=")
			sec, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				return 0, false
			}
			return sec, true
		}
	}
	return 0, false
}
