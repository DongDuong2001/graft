// Package observability provides security event logging, metrics, and monitoring.
package observability

import (
	"encoding/json"
	"log/slog"
	"os"
	"time"
)

// EventType categorizes security audit events.
type EventType string

const (
	// Auth events
	EventAuthSuccess EventType = "auth.success"
	EventAuthFailure EventType = "auth.failure"
	EventAuthLockout EventType = "auth.lockout"

	// Access events
	EventAccessDenied    EventType = "access.denied"
	EventRateLimitHit    EventType = "ratelimit.hit"
	EventIPBlocked       EventType = "ip.blocked"
	EventCIDRBlocked     EventType = "cidr.blocked"

	// Webhook events
	EventWebhookReceived   EventType = "webhook.received"
	EventWebhookDelivered  EventType = "webhook.delivered"
	EventWebhookFailed     EventType = "webhook.failed"
	EventSignatureInvalid  EventType = "signature.invalid"
	EventReplayDetected    EventType = "replay.detected"

	// Rule events
	EventRuleCreated EventType = "rule.created"
	EventRuleUpdated EventType = "rule.updated"
	EventRuleDeleted EventType = "rule.deleted"

	// System events
	EventConfigReload EventType = "config.reload"
	EventStartup      EventType = "system.startup"
	EventShutdown     EventType = "system.shutdown"
)

// Severity indicates the importance of the event.
type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warning"
	SeverityCritical Severity = "critical"
)

// Event represents a single audit log entry.
type Event struct {
	Timestamp   time.Time       `json:"timestamp"`
	Type        EventType       `json:"type"`
	Severity    Severity        `json:"severity"`
	ClientIP    string          `json:"client_ip,omitempty"`
	UserID      string          `json:"user_id,omitempty"`
	Resource    string          `json:"resource,omitempty"`
	Action      string          `json:"action,omitempty"`
	Success     bool            `json:"success,omitempty"`
	Details     json.RawMessage `json:"details,omitempty"`
	RequestID   string          `json:"request_id,omitempty"`
}

// Logger handles security audit logging.
type Logger struct {
	logger *slog.Logger
	file   *os.File
}

// Config configures the audit logger.
type Config struct {
	// Enabled enables audit logging
	Enabled bool
	// FilePath is the path to the audit log file (stdout if empty)
	FilePath string
	// MinSeverity is the minimum severity to log (info, warning, critical)
	MinSeverity string
}

// NewLogger creates a new audit logger.
func NewLogger(cfg Config) (*Logger, error) {
	if !cfg.Enabled {
		return &Logger{logger: slog.New(slog.DiscardHandler)}, nil
	}

	var handler slog.Handler
	opts := &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}

	if cfg.FilePath != "" {
		f, err := os.OpenFile(cfg.FilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
		if err != nil {
			return nil, err
		}
		handler = slog.NewJSONHandler(f, opts)
		return &Logger{
			logger: slog.New(handler),
			file:   f,
		}, nil
	}

	handler = slog.NewJSONHandler(os.Stdout, opts)
	return &Logger{logger: slog.New(handler)}, nil
}

// Close closes the audit log file.
func (l *Logger) Close() error {
	if l.file != nil {
		return l.file.Close()
	}
	return nil
}

// Log records a security audit event.
func (l *Logger) Log(evt Event) {
	attrs := []slog.Attr{
		slog.String("event_type", string(evt.Type)),
		slog.String("severity", string(evt.Severity)),
		slog.Time("timestamp", evt.Timestamp),
	}

	if evt.ClientIP != "" {
		attrs = append(attrs, slog.String("client_ip", evt.ClientIP))
	}
	if evt.UserID != "" {
		attrs = append(attrs, slog.String("user_id", evt.UserID))
	}
	if evt.Resource != "" {
		attrs = append(attrs, slog.String("resource", evt.Resource))
	}
	if evt.Action != "" {
		attrs = append(attrs, slog.String("action", evt.Action))
	}
	if evt.RequestID != "" {
		attrs = append(attrs, slog.String("request_id", evt.RequestID))
	}

	switch evt.Severity {
	case SeverityCritical:
		l.logger.LogAttrs(nil, slog.LevelError, "SECURITY_EVENT", attrs...)
	case SeverityWarning:
		l.logger.LogAttrs(nil, slog.LevelWarn, "SECURITY_EVENT", attrs...)
	default:
		l.logger.LogAttrs(nil, slog.LevelInfo, "SECURITY_EVENT", attrs...)
	}
}

// Convenience methods

func (l *Logger) AuthSuccess(clientIP, userID string) {
	l.Log(Event{
		Timestamp: time.Now(),
		Type:      EventAuthSuccess,
		Severity:  SeverityInfo,
		ClientIP:  clientIP,
		UserID:    userID,
		Success:   true,
	})
}

func (l *Logger) AuthFailure(clientIP, reason string) {
	l.Log(Event{
		Timestamp: time.Now(),
		Type:      EventAuthFailure,
		Severity:  SeverityWarning,
		ClientIP:  clientIP,
		Action:    reason,
		Success:   false,
	})
}

func (l *Logger) AuthLockout(clientIP string, duration time.Duration) {
	l.Log(Event{
		Timestamp: time.Now(),
		Type:      EventAuthLockout,
		Severity:  SeverityWarning,
		ClientIP:  clientIP,
		Action:    "locked",
		Resource:  duration.String(),
	})
}

func (l *Logger) AccessDenied(clientIP, resource string) {
	l.Log(Event{
		Timestamp: time.Now(),
		Type:      EventAccessDenied,
		Severity:  SeverityWarning,
		ClientIP:  clientIP,
		Resource:  resource,
	})
}

func (l *Logger) RateLimitHit(clientIP, resource string) {
	l.Log(Event{
		Timestamp: time.Now(),
		Type:      EventRateLimitHit,
		Severity:  SeverityWarning,
		ClientIP:  clientIP,
		Resource:  resource,
	})
}

func (l *Logger) SignatureInvalid(clientIP, ruleID string) {
	l.Log(Event{
		Timestamp: time.Now(),
		Type:      EventSignatureInvalid,
		Severity:  SeverityWarning,
		ClientIP:  clientIP,
		Resource:  ruleID,
	})
}

func (l *Logger) ReplayDetected(clientIP, ruleID string) {
	l.Log(Event{
		Timestamp: time.Now(),
		Type:      EventReplayDetected,
		Severity:  SeverityCritical,
		ClientIP:  clientIP,
		Resource:  ruleID,
	})
}

func (l *Logger) RuleChange(clientIP, userID string, eventType EventType, ruleID string) {
	l.Log(Event{
		Timestamp: time.Now(),
		Type:      eventType,
		Severity:  SeverityInfo,
		ClientIP:  clientIP,
		UserID:    userID,
		Resource:  ruleID,
	})
}

// Global singleton for easy access (initialized in main).
var defaultLogger *Logger

// SetDefaultLogger sets the global audit logger.
func SetDefaultLogger(l *Logger) {
	defaultLogger = l
}

// Default returns the global audit logger.
func Default() *Logger {
	if defaultLogger == nil {
		return &Logger{logger: slog.New(slog.DiscardHandler)}
	}
	return defaultLogger
}
