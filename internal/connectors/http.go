package connectors

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"Graft/internal/models"
	"Graft/internal/observability"
)

// HTTPConfig controls outbound HTTP delivery (retries and timeouts).
type HTTPConfig struct {
	Timeout       time.Duration
	MaxRetries    int
	BaseRetryWait time.Duration
}

// HTTPForwarder POST/PUT/etc. to rule.DestinationURL with retries on transient failures.
type HTTPForwarder struct {
	client *http.Client
	cfg    HTTPConfig
}

// NewHTTPForwarder builds an HTTPForwarder with defaults.
func NewHTTPForwarder(cfg HTTPConfig) *HTTPForwarder {
	if cfg.Timeout <= 0 {
		cfg.Timeout = 30 * time.Second
	}
	if cfg.MaxRetries < 0 {
		cfg.MaxRetries = 0
	}
	if cfg.BaseRetryWait <= 0 {
		cfg.BaseRetryWait = 200 * time.Millisecond
	}
	return &HTTPForwarder{
		client: &http.Client{Timeout: cfg.Timeout},
		cfg:    cfg,
	}
}

// Send delivers payload and returns the last HTTP status, attempt count, and error if delivery failed.
func (f *HTTPForwarder) Send(ctx context.Context, rule *models.Rule, payload []byte) (statusCode int, attempts int, err error) {
	if rule.DestinationURL == "" {
		return 0, 0, fmt.Errorf("destination URL is empty")
	}

	method := rule.DestinationMethod
	if method == "" {
		method = http.MethodPost
	}

	maxAttempts := 1 + f.cfg.MaxRetries
	var lastCode int
	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			// Exponential backoff: Base * 2^(attempt-1)
			delay := f.cfg.BaseRetryWait * time.Duration(1<<uint(attempt-1))
			// Add jitter: ±20% variation
			jitter := time.Duration(rand.Int63n(int64(delay) / 5))
			if rand.Intn(2) == 0 {
				delay += jitter
			} else {
				delay -= jitter
			}

			t := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				t.Stop()
				return lastCode, attempt, ctx.Err()
			case <-t.C:
			}
		}

		code, retry, err := f.doOnce(ctx, method, rule, payload)
		lastCode = code
		lastErr = err
		attempts = attempt + 1
		if err == nil {
			return code, attempts, nil
		}
		if !retry || attempt == maxAttempts-1 {
			break
		}
	}
	return lastCode, attempts, lastErr
}

func (f *HTTPForwarder) doOnce(ctx context.Context, method string, rule *models.Rule, payload []byte) (code int, retry bool, err error) {
	observability.AddForwards(1)

	req, err := http.NewRequestWithContext(ctx, method, rule.DestinationURL, bytes.NewReader(payload))
	if err != nil {
		return 0, false, err
	}

	req.Header.Set("Content-Type", "application/json")
	for k, v := range rule.DestinationHeaders {
		req.Header.Set(k, v)
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return 0, isRetryableNetErr(err), fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return resp.StatusCode, false, nil
	}

	retry = resp.StatusCode == http.StatusTooManyRequests ||
		resp.StatusCode == http.StatusBadGateway ||
		resp.StatusCode == http.StatusServiceUnavailable ||
		resp.StatusCode == http.StatusGatewayTimeout
	return resp.StatusCode, retry, fmt.Errorf("received non-2xx response: %d", resp.StatusCode)
}

func isRetryableNetErr(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "timeout") ||
		strings.Contains(s, "temporary") ||
		strings.Contains(s, "connection reset") ||
		strings.Contains(s, "eof")
}
