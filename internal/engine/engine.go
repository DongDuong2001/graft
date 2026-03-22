package engine

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"Graft/internal/crypto"
	"Graft/internal/forwarder"
	"Graft/internal/metrics"
	"Graft/internal/models"
	"Graft/internal/storage"
	"Graft/internal/transformer"
	"Graft/internal/webhook"
)

// Factory defines the processing logic.
type Engine struct {
	repo        storage.Repository
	masterKey   string
	transformer *transformer.Transformer
	forwarder   forwarder.Forwarder
}

// New creates a new engine.
func New(repo storage.Repository, masterKey string, fwd forwarder.Forwarder) *Engine {
	return &Engine{
		repo:        repo,
		masterKey:   masterKey,
		transformer: transformer.New(),
		forwarder:   fwd,
	}
}

// Process handles an incoming webhook: lookup, verify, transform, forward.
func (e *Engine) Process(ctx context.Context, wh *webhook.Webhook) (*models.Delivery, error) {
	rule, err := e.repo.GetRuleByPath(ctx, wh.Path)
	if err != nil {
		return nil, fmt.Errorf("lookup rule: %w", err)
	}
	if rule == nil {
		return nil, models.ErrRuleNotFound
	}

	metrics.IncWebhooksReceived()

	// 1. Verify Signature
	if rule.RequiredSignature {
		secret, err := crypto.Decrypt(rule.SignatureSecret, e.masterKey)
		if err != nil {
			metrics.IncWebhooksFailed()
			return nil, fmt.Errorf("decrypt secret: %w", err)
		}
		if err := wh.VerifySignature(rule, secret); err != nil {
			metrics.IncWebhooksFailed()
			// Return special error to indicate unauthorized?
			return nil, fmt.Errorf("%w: %v", models.ErrUnauthorized, err)
		}
	} else {
		// Even if signature not required, if header provided we "could" check it, but logic says no.
	}

	// 2. Transform
	transformedBody, err := e.transformer.Transform(wh.Body, rule.TransformTemplate)
	if err != nil {
		metrics.IncWebhooksFailed()
		return nil, fmt.Errorf("transform: %w", err)
	}

	// 3. Forward
	start := time.Now()
	status, attempts, fwdErr := e.forwarder.Send(ctx, rule, transformedBody)
	duration := time.Since(start).Milliseconds()

	retryCount := attempts - 1
	if retryCount < 0 {
		retryCount = 0
	}

	// 4. Record Delivery
	d := models.Delivery{
		ID:         newID(),
		RuleID:     rule.ID,
		CreatedAt:  time.Now().Unix(),
		Success:    fwdErr == nil,
		StatusCode: status,
		DurationMS: duration,
		RetryCount: retryCount,
	}
	if fwdErr != nil {
		d.ErrorMsg = fwdErr.Error()
	}

	if err := e.repo.SaveDelivery(ctx, d); err != nil {
		// We log this but return the result of the forward operation?
		// Realistically we should probably log it here.
	}

	if fwdErr != nil {
		metrics.IncWebhooksFailed()
		return &d, fwdErr
	}

	metrics.IncWebhooksSuccess()
	metrics.AddForwards(1)
	return &d, nil
}

func newID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// Ensure models has these errors
func init() {
	if models.ErrRuleNotFound == nil {
		models.ErrRuleNotFound = fmt.Errorf("rule not found")
	}
	if models.ErrUnauthorized == nil {
		models.ErrUnauthorized = fmt.Errorf("unauthorized")
	}
}
