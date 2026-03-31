package admin

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"

	"Graft/internal/crypto"
	"Graft/internal/models"
	"Graft/internal/storage"
)

// Service handles rule management.
type Service struct {
	repo      storage.Repository
	masterKey string
}

// NewService creates an admin service.
func NewService(repo storage.Repository, masterKey string) *Service {
	return &Service{
		repo:      repo,
		masterKey: masterKey,
	}
}

// --- RuleInput represents fields for creating or updating a rule ---
type RuleInput struct {
	Name                     string
	Description              string
	ListenPath               string
	RequiredSignature        bool
	SignatureHeader          string
	SignatureFormat          string
	SignatureTimestampHeader string
	SignatureMaxSkewSeconds  int
	SignatureSecret          string // Plaintext secret provided by user
	ClearSignatureSecret     bool   // Request to clear the secret (update only)
	TransformTemplate        string
	DestinationURL           string
	DestinationMethod        string
	DestinationHeaders       map[string]string

	// --- Fan-out: multiple destinations ---
	Destinations []models.Destination

	// --- Conditional routing: payload-based expressions ---
	Conditions []models.Condition

	// --- Pipeline: multi-step transformations ---
	TransformSteps []models.TransformStep

	// --- Per-rule rate limiting ---
	RateLimitMax    int
	RateLimitWindow string

	// --- Per-rule IP allowlist (CIDR notation) ---
	IPAllowlist []string
}

// --- CreateRule validates input and creates a new rule ---
func (s *Service) CreateRule(ctx context.Context, in RuleInput) (*models.Rule, error) {
	if err := s.validate(in, true); err != nil {
		return nil, err
	}

	id := newID()
	rule := models.Rule{
		ID:                       id,
		Name:                     strings.TrimSpace(in.Name),
		Description:              in.Description,
		ListenPath:               strings.TrimSpace(in.ListenPath),
		RequiredSignature:        in.RequiredSignature,
		SignatureHeader:          strings.TrimSpace(in.SignatureHeader),
		SignatureFormat:          in.SignatureFormat,
		SignatureTimestampHeader: strings.TrimSpace(in.SignatureTimestampHeader),
		SignatureMaxSkewSeconds:  in.SignatureMaxSkewSeconds,
		TransformTemplate:        in.TransformTemplate,
		DestinationURL:           strings.TrimSpace(in.DestinationURL),
		DestinationMethod:        strings.TrimSpace(in.DestinationMethod),
		DestinationHeaders:       in.DestinationHeaders,
		// --- New fields ---
		Destinations:    in.Destinations,
		Conditions:      in.Conditions,
		TransformSteps:  in.TransformSteps,
		RateLimitMax:    in.RateLimitMax,
		RateLimitWindow: in.RateLimitWindow,
		IPAllowlist:     in.IPAllowlist,
	}

	if rule.SignatureFormat == "" {
		rule.SignatureFormat = "hex"
	}
	if rule.DestinationHeaders == nil {
		rule.DestinationHeaders = map[string]string{}
	}

	// --- Encrypt Secret ---
	if in.RequiredSignature && in.SignatureSecret != "" {
		enc, err := crypto.Encrypt(in.SignatureSecret, s.masterKey)
		if err != nil {
			return nil, fmt.Errorf("encrypt secret: %w", err)
		}
		rule.SignatureSecret = enc
	}

	if err := s.repo.SaveRule(ctx, rule); err != nil {
		return nil, err
	}

	return &rule, nil
}

// --- UpdateRule updates an existing rule ---
func (s *Service) UpdateRule(ctx context.Context, id string, in RuleInput) (*models.Rule, error) {
	existing, err := s.repo.GetRuleByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, models.ErrRuleNotFound
	}

	if err := s.validate(in, false); err != nil {
		return nil, err
	}

	rule := *existing
	rule.Name = strings.TrimSpace(in.Name)
	rule.Description = in.Description
	rule.ListenPath = strings.TrimSpace(in.ListenPath)
	rule.RequiredSignature = in.RequiredSignature
	rule.SignatureHeader = strings.TrimSpace(in.SignatureHeader)
	rule.SignatureFormat = in.SignatureFormat
	rule.SignatureTimestampHeader = strings.TrimSpace(in.SignatureTimestampHeader)
	rule.SignatureMaxSkewSeconds = in.SignatureMaxSkewSeconds
	rule.TransformTemplate = in.TransformTemplate
	rule.DestinationURL = strings.TrimSpace(in.DestinationURL)
	rule.DestinationMethod = strings.TrimSpace(in.DestinationMethod)

	if in.DestinationHeaders != nil {
		rule.DestinationHeaders = in.DestinationHeaders
	}

	// --- Update new fields ---
	rule.Destinations = in.Destinations
	rule.Conditions = in.Conditions
	rule.TransformSteps = in.TransformSteps
	rule.RateLimitMax = in.RateLimitMax
	rule.RateLimitWindow = in.RateLimitWindow
	rule.IPAllowlist = in.IPAllowlist

	if rule.SignatureFormat == "" {
		rule.SignatureFormat = "hex"
	}

	// --- Handle Secret Update ---
	if in.ClearSignatureSecret {
		if rule.RequiredSignature {
			return nil, fmt.Errorf("cannot clear signature secret when signature is required")
		}
		rule.SignatureSecret = ""
	} else if strings.TrimSpace(in.SignatureSecret) != "" {
		enc, err := crypto.Encrypt(in.SignatureSecret, s.masterKey)
		if err != nil {
			return nil, fmt.Errorf("encrypt secret: %w", err)
		}
		rule.SignatureSecret = enc
	} else if rule.RequiredSignature && rule.SignatureSecret == "" {
		return nil, fmt.Errorf("signature secret is required")
	}

	if err := s.repo.SaveRule(ctx, rule); err != nil {
		return nil, err
	}

	return &rule, nil
}

func (s *Service) DeleteRule(ctx context.Context, id string) error {
	return s.repo.DeleteRule(ctx, id)
}

func (s *Service) GetRule(ctx context.Context, id string) (*models.Rule, error) {
	return s.repo.GetRuleByID(ctx, id)
}

func (s *Service) ListRules(ctx context.Context) ([]models.Rule, error) {
	return s.repo.ListRules(ctx)
}

func (s *Service) ListDeliveries(ctx context.Context, ruleID string, limit int) ([]models.Delivery, error) {
	return s.repo.ListDeliveriesByRule(ctx, ruleID, limit)
}

// --- validate ensures rule inputs are well-formed ---
func (s *Service) validate(in RuleInput, isCreate bool) error {
	if strings.TrimSpace(in.Name) == "" {
		return fmt.Errorf("name is required")
	}
	if !strings.HasPrefix(strings.TrimSpace(in.ListenPath), "/hook/") {
		return fmt.Errorf("listen_path must start with /hook/")
	}
	// --- Allow empty destination_url if Destinations slice is populated ---
	if strings.TrimSpace(in.DestinationURL) == "" && len(in.Destinations) == 0 {
		return fmt.Errorf("destination_url or destinations is required")
	}

	if in.RequiredSignature {
		if strings.TrimSpace(in.SignatureHeader) == "" {
			return fmt.Errorf("signature_header is required when required_signature is true")
		}
		if isCreate && strings.TrimSpace(in.SignatureSecret) == "" {
			return fmt.Errorf("signature_secret is required for new rule with required_signature")
		}
	}

	// --- Validate signature format (expanded provider support) ---
	validFormats := map[string]bool{
		"": true, "hex": true, "stripe_v1": true,
		"shopify_hmac": true, "slack_v0": true, "twilio": true, "pagerduty": true,
	}
	if !validFormats[in.SignatureFormat] {
		return fmt.Errorf("signature_format must be one of: hex, stripe_v1, shopify_hmac, slack_v0, twilio, pagerduty")
	}

	// --- Validate transform step types ---
	for i, step := range in.TransformSteps {
		if step.Type != "go_template" && step.Type != "javascript" {
			return fmt.Errorf("transform_steps[%d]: type must be go_template or javascript", i)
		}
		if step.Script == "" {
			return fmt.Errorf("transform_steps[%d]: script is required", i)
		}
	}

	// --- Validate condition operators ---
	for i, cond := range in.Conditions {
		validOps := map[string]bool{"eq": true, "neq": true, "contains": true, "exists": true}
		if !validOps[cond.Operator] {
			return fmt.Errorf("conditions[%d]: operator must be eq, neq, contains, or exists", i)
		}
	}

	return nil
}

func newID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func init() {
	if models.ErrRuleNotFound == nil {
		models.ErrRuleNotFound = fmt.Errorf("rule not found")
	}
}
