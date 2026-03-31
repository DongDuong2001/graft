package engine

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"Graft/internal/connectors"
	"Graft/internal/crypto"
	"Graft/internal/forwarder"
	"Graft/internal/metrics"
	"Graft/internal/models"
	"Graft/internal/storage"
	"Graft/internal/transformer"
	"Graft/internal/webhook"
	"log/slog"
)

// Engine defines the processing logic.
type Engine struct {
	repo        storage.Repository
	masterKey   string
	transformer *transformer.Transformer
	forwarder   forwarder.Forwarder
	registry    *connectors.Registry
}

// New creates a new engine.
func New(repo storage.Repository, masterKey string, fwd forwarder.Forwarder, reg *connectors.Registry) *Engine {
	return &Engine{
		repo:        repo,
		masterKey:   masterKey,
		transformer: transformer.New(),
		forwarder:   fwd,
		registry:    reg,
	}
}

// PrepareDelivery creates a pending delivery record for a webhook.
func (e *Engine) PrepareDelivery(ctx context.Context, wh *webhook.Webhook) (*models.Delivery, *models.Rule, error) {
	rule, err := e.repo.GetRuleByPath(ctx, wh.Path)
	if err != nil {
		return nil, nil, fmt.Errorf("lookup rule: %w", err)
	}
	if rule == nil {
		return nil, nil, models.ErrRuleNotFound
	}

	d := models.Delivery{
		ID:          GenerateID(),
		RuleID:      rule.ID,
		CreatedAt:   time.Now().Unix(),
		Status:      models.StatusPending,
		RequestBody: wh.Body,
		RequestPath: wh.Path,
	}

	if err := e.repo.SaveDelivery(ctx, d); err != nil {
		return nil, nil, fmt.Errorf("save initial delivery: %w", err)
	}

	return &d, rule, nil
}

// Process handles an incoming webhook: lookup, verify, transform, forward.
// If d is provided, it updates that delivery record.
func (e *Engine) Process(ctx context.Context, wh *webhook.Webhook) (*models.Delivery, error) {
	rule, err := e.repo.GetRuleByPath(ctx, wh.Path)
	if err != nil {
		return nil, fmt.Errorf("lookup rule: %w", err)
	}
	if rule == nil {
		return nil, models.ErrRuleNotFound
	}

	d := &models.Delivery{
		ID:        GenerateID(),
		RuleID:    rule.ID,
		CreatedAt: time.Now().Unix(),
		Status:    models.StatusProcessing,
	}
	// Note: In worker-based flow, we usually already have a DeliveryID and record.
	// This Process method is kept compatible for both sync and async.

	return e.processWithDelivery(ctx, rule, wh, d)
}

// ProcessAsync is called by background workers. It expects a pre-created delivery record.
func (e *Engine) ProcessAsync(ctx context.Context, deliveryID string, wh *webhook.Webhook) error {
	rule, err := e.repo.GetRuleByPath(ctx, wh.Path)
	if err != nil {
		_ = e.repo.UpdateDeliveryStatus(ctx, deliveryID, models.StatusFailed, err.Error())
		return err
	}
	if rule == nil {
		_ = e.repo.UpdateDeliveryStatus(ctx, deliveryID, models.StatusFailed, "rule not found")
		return models.ErrRuleNotFound
	}

	d := &models.Delivery{
		ID:        deliveryID,
		RuleID:    rule.ID,
		CreatedAt: time.Now().Unix(), // Should ideally be from original task
		Status:    models.StatusProcessing,
	}

	_, err = e.processWithDelivery(ctx, rule, wh, d)
	return err
}

// --- processWithDelivery: core processing pipeline (verify → condition → transform → fan-out) ---
func (e *Engine) processWithDelivery(ctx context.Context, rule *models.Rule, wh *webhook.Webhook, d *models.Delivery) (*models.Delivery, error) {
	metrics.IncWebhooksReceived()
	_ = e.repo.UpdateDeliveryStatus(ctx, d.ID, models.StatusProcessing, "")

	// --- Step 1: Verify Signature ---
	if rule.RequiredSignature {
		secret, err := crypto.Decrypt(rule.SignatureSecret, e.masterKey)
		if err != nil {
			metrics.IncWebhooksFailed()
			_ = e.repo.UpdateDeliveryStatus(ctx, d.ID, models.StatusFailed, err.Error())
			return nil, fmt.Errorf("decrypt secret: %w", err)
		}
		if err := wh.VerifySignature(rule, secret); err != nil {
			metrics.IncWebhooksFailed()
			_ = e.repo.UpdateDeliveryStatus(ctx, d.ID, models.StatusFailed, err.Error())
			return nil, fmt.Errorf("%w: %v", models.ErrUnauthorized, err)
		}
	}

	// --- Step 2: Evaluate Conditions (conditional routing) ---
	if len(rule.Conditions) > 0 {
		pass, reason := EvaluateConditions(wh.Body, rule.Conditions)
		if !pass {
			slog.Info("Webhook skipped by condition", "rule", rule.Name, "reason", reason)
			d.Status = models.StatusDelivered
			d.Success = true
			d.ErrorMsg = "skipped: " + reason
			_ = e.repo.SaveDelivery(ctx, *d)
			return d, nil
		}
	}

	// --- Step 3: Transform payload (pipeline or legacy single template) ---
	transformedBody, err := e.transformer.TransformPipeline(wh.Body, rule.TransformSteps, rule.TransformTemplate)
	if err != nil {
		metrics.IncWebhooksFailed()
		_ = e.repo.UpdateDeliveryStatus(ctx, d.ID, models.StatusFailed, err.Error())
		return nil, fmt.Errorf("transform: %w", err)
	}

	// --- Step 4: Fan-out — send to all effective destinations ---
	destinations := rule.EffectiveDestinations()
	if len(destinations) == 0 {
		d.Status = models.StatusFailed
		d.ErrorMsg = "no destinations configured"
		_ = e.repo.SaveDelivery(ctx, *d)
		metrics.IncWebhooksFailed()
		return d, fmt.Errorf("no destinations configured for rule %q", rule.Name)
	}

	// --- Fan-out: for multiple destinations, send concurrently ---
	var (
		lastStatus   int
		lastAttempts int
		lastErr      error
		totalStart   = time.Now()
	)

	for _, dest := range destinations {
		// --- Check per-destination condition (optional gate) ---
		if dest.Condition != "" {
			cond := models.Condition{Field: dest.Condition, Operator: "exists"}
			if pass, _ := EvaluateConditions(wh.Body, []models.Condition{cond}); !pass {
				slog.Debug("Skipping destination by condition", "url", dest.URL, "condition", dest.Condition)
				continue
			}
		}

		// --- Route logic based on Destination Type ---
		if dest.Type != "" && dest.Type != "http" {
			// Native connector route
			if native := e.registry.Get(dest.Type); native != nil {
				status, nativeErr := native.Send(ctx, dest.URL, transformedBody)
				lastStatus = status
				lastAttempts = 1 // Single attempt for native without retry wrapper loop for now
				if nativeErr != nil {
					lastErr = nativeErr
					slog.Error("Native fan-out failed", "type", dest.Type, "dest", dest.URL, "status", status, "error", nativeErr)
				}
				continue
			}
			slog.Warn("Unknown destination type, falling back to HTTP", "type", dest.Type)
		}

		// --- Standard HTTP Forwarder Route ---
		// Build a temporary rule-like struct for the standard forwarder
		destRule := &models.Rule{
			DestinationURL:     dest.URL,
			DestinationMethod:  dest.Method,
			DestinationHeaders: dest.Headers,
		}
		if destRule.DestinationHeaders == nil {
			destRule.DestinationHeaders = map[string]string{}
		}

		status, attempts, fwdErr := e.forwarder.Send(ctx, destRule, transformedBody)
		lastStatus = status
		lastAttempts = attempts
		if fwdErr != nil {
			lastErr = fwdErr
			slog.Error("Fan-out delivery failed", "dest", dest.URL, "status", status, "error", fwdErr)
		}
	}

	// --- Step 5: Record delivery result ---
	duration := time.Since(totalStart).Milliseconds()
	d.Success = lastErr == nil
	d.StatusCode = lastStatus
	d.DurationMS = duration
	d.RetryCount = lastAttempts - 1
	if d.RetryCount < 0 {
		d.RetryCount = 0
	}
	if lastErr != nil {
		d.ErrorMsg = lastErr.Error()
		// --- Mark as dead letter if forwarding failed after all retries ---
		d.Status = models.StatusDeadLetter
	} else {
		d.Status = models.StatusDelivered
	}

	if err := e.repo.SaveDelivery(ctx, *d); err != nil {
		slog.Error("Failed to save delivery result", "id", d.ID, "error", err)
	}

	if lastErr != nil {
		metrics.IncWebhooksFailed()
		return d, lastErr
	}

	metrics.IncWebhooksSuccess()
	metrics.AddForwards(uint64(len(destinations)))
	return d, nil
}

// GenerateID creates a new unique ID.
func GenerateID() string {
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

// ---------------------------------------------------------------------------
// ReplayDelivery re-runs a delivery using its captured request payload.
// Used by the admin API to manually trigger retries.
// ---------------------------------------------------------------------------
func (e *Engine) ReplayDelivery(ctx context.Context, deliveryID string) (*models.Delivery, error) {
	// --- Fetch the existing delivery and its payload ---
	d, err := e.repo.GetDeliveryByID(ctx, deliveryID)
	if err != nil {
		return nil, fmt.Errorf("lookup delivery: %w", err)
	}
	if d == nil {
		return nil, fmt.Errorf("delivery not found")
	}

	if len(d.RequestBody) == 0 {
		return nil, fmt.Errorf("delivery %s has no captured request body; cannot replay", deliveryID)
	}

	// --- Reconstruct a mock webhook ---
	wh := &webhook.Webhook{
		Path:    d.RequestPath,
		Headers: nil, // Note: raw signature headers are not captured; replay assumes manual authorization bypasses strict signature check if missing?
		Body:    d.RequestBody,
	}

	// --- Fetch current rule for this path ---
	rule, err := e.repo.GetRuleByPath(ctx, d.RequestPath)
	if err != nil {
		return nil, fmt.Errorf("lookup rule for replay: %w", err)
	}
	if rule == nil {
		return nil, models.ErrRuleNotFound
	}

	// --- Process webhook reusing existing delivery ---
	return e.processWithDelivery(ctx, rule, wh, d)
}
