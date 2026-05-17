package engine

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/DongDuong2001/graft/internal/connectors"
	"github.com/DongDuong2001/graft/internal/crypto"
	"github.com/DongDuong2001/graft/internal/forwarder"
	"github.com/DongDuong2001/graft/internal/metrics"
	"github.com/DongDuong2001/graft/internal/models"
	"github.com/DongDuong2001/graft/internal/storage"
	"github.com/DongDuong2001/graft/internal/transformer"
	"github.com/DongDuong2001/graft/internal/webhook"
)

const maxFanOutConcurrency = 8

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
	totalStart := time.Now()
	results := e.deliverFanOut(ctx, destinations, wh.Body, transformedBody)
	summary := summarizeFanOut(results)

	// --- Step 5: Record delivery result ---
	duration := time.Since(totalStart).Milliseconds()
	d.Success = summary.success
	d.StatusCode = summary.statusCode
	d.DurationMS = duration
	d.RetryCount = summary.retryCount
	d.ErrorMsg = summary.errorMessage
	d.Status = summary.status

	if err := e.repo.SaveDelivery(ctx, *d); err != nil {
		slog.Error("Failed to save delivery result", "id", d.ID, "error", err)
	}

	metrics.AddForwards(uint64(summary.attempted))
	if summary.err != nil {
		metrics.IncWebhooksFailed()
		return d, summary.err
	}

	metrics.IncWebhooksSuccess()
	return d, nil
}

type fanOutResult struct {
	index      int
	dest       models.Destination
	statusCode int
	attempts   int
	skipped    bool
	err        error
}

type fanOutSummary struct {
	success      bool
	status       string
	statusCode   int
	retryCount   int
	attempted    int
	errorMessage string
	err          error
}

func (e *Engine) deliverFanOut(ctx context.Context, destinations []models.Destination, originalBody, transformedBody []byte) []fanOutResult {
	results := make([]fanOutResult, len(destinations))
	for i, dest := range destinations {
		results[i] = fanOutResult{index: i, dest: dest}
	}

	limit := len(destinations)
	if limit > maxFanOutConcurrency {
		limit = maxFanOutConcurrency
	}
	if limit <= 0 {
		return results
	}

	var wg sync.WaitGroup
	sem := make(chan struct{}, limit)

	for i, dest := range destinations {
		if shouldSkipDestination(dest, originalBody) {
			results[i].skipped = true
			slog.Debug("Skipping destination by condition", "destination_index", i, "condition", dest.Condition)
			continue
		}

		wg.Add(1)
		go func(i int, dest models.Destination) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				results[i].err = ctx.Err()
				return
			}
			results[i] = e.sendDestination(ctx, i, dest, transformedBody)
		}(i, dest)
	}

	wg.Wait()
	return results
}

func shouldSkipDestination(dest models.Destination, body []byte) bool {
	if dest.Condition == "" {
		return false
	}
	cond := models.Condition{Field: dest.Condition, Operator: "exists"}
	pass, _ := EvaluateConditions(body, []models.Condition{cond})
	return !pass
}

func (e *Engine) sendDestination(ctx context.Context, index int, dest models.Destination, transformedBody []byte) fanOutResult {
	result := fanOutResult{index: index, dest: dest}

	if dest.Type != "" && dest.Type != "http" {
		if e.registry != nil {
			if native := e.registry.Get(dest.Type); native != nil {
				status, err := native.Send(ctx, dest.URL, transformedBody)
				result.statusCode = status
				result.attempts = 1
				result.err = err
				if err != nil {
					slog.Error("Native fan-out failed", "destination_index", index, "type", dest.Type, "status", status, "error", err)
				}
				return result
			}
		}
		slog.Warn("Unknown destination type, falling back to HTTP", "destination_index", index, "type", dest.Type)
	}

	destRule := &models.Rule{
		DestinationURL:     dest.URL,
		DestinationMethod:  dest.Method,
		DestinationHeaders: dest.Headers,
	}
	if destRule.DestinationHeaders == nil {
		destRule.DestinationHeaders = map[string]string{}
	}

	status, attempts, err := e.forwarder.Send(ctx, destRule, transformedBody)
	result.statusCode = status
	result.attempts = attempts
	result.err = err
	if err != nil {
		slog.Error("Fan-out delivery failed", "destination_index", index, "status", status, "error", err)
	}
	return result
}

func summarizeFanOut(results []fanOutResult) fanOutSummary {
	summary := fanOutSummary{
		success: true,
		status:  models.StatusDelivered,
	}

	var (
		successes int
		failures  []string
	)

	for _, result := range results {
		if result.skipped {
			continue
		}

		summary.attempted++
		if result.statusCode != 0 {
			summary.statusCode = result.statusCode
		}
		if result.attempts > 1 {
			summary.retryCount += result.attempts - 1
		}

		if result.err != nil {
			failures = append(failures, fmt.Sprintf("destination %d failed: %v", result.index+1, result.err))
			continue
		}
		successes++
	}

	if summary.attempted == 0 {
		summary.errorMessage = "skipped: no destination conditions matched"
		return summary
	}
	if len(failures) == 0 {
		return summary
	}

	summary.success = false
	summary.errorMessage = strings.Join(failures, "; ")
	summary.err = fmt.Errorf("fan-out delivery failed: %s", summary.errorMessage)
	if successes > 0 {
		summary.status = models.StatusPartial
	} else {
		summary.status = models.StatusDeadLetter
	}
	return summary
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
