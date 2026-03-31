package connectors

import (
	"context"
	"strings"
)

// ---------------------------------------------------------------------------
// NativeConnector defines the interface for native integration connectors.
// Each connector formats and delivers JSON payloads to a specific third-party
// vendor (e.g. Slack, Discord, MS Teams, SMTP).
// ---------------------------------------------------------------------------
type NativeConnector interface {
	Send(ctx context.Context, url string, payload []byte) (statusCode int, err error)
}

// ---------------------------------------------------------------------------
// Registry maintains a mapping of integration types to their respective
// NativeConnectors. It uses the "type" field from the rule's destination
// configuration to route the payload appropriately.
// ---------------------------------------------------------------------------
type Registry struct {
	connectors map[string]NativeConnector
}

// --- NewRegistry initializes a registry with the built-in connectors ---
func NewRegistry(email *EmailConnector) *Registry {
	r := &Registry{
		connectors: make(map[string]NativeConnector),
	}

	// --- Register standard native connectors ---
	r.Register("slack", NewSlackConnector())
	r.Register("discord", NewDiscordConnector())
	r.Register("teams", NewTeamsConnector())

	// --- Register configurable connectors (like Email) if provided ---
	// Note: email connector uses "URL" as "To" address
	if email != nil {
		// Wrap email connector to match NativeConnector interface
		r.Register("email", &emailAdapter{email: email})
	}

	return r
}

// --- Register adds or overrides a connector for a given type ---
func (r *Registry) Register(typ string, c NativeConnector) {
	r.connectors[strings.ToLower(typ)] = c
}

// --- Get retrieves a connector by type. Returns nil if not found ---
func (r *Registry) Get(typ string) NativeConnector {
	if c, ok := r.connectors[strings.ToLower(typ)]; ok {
		return c
	}
	return nil
}

// ---------------------------------------------------------------------------
// emailAdapter adapts the EmailConnector to the NativeConnector interface
// by treating the 'url' field as the 'To' address and providing a default subject.
// ---------------------------------------------------------------------------
type emailAdapter struct {
	email *EmailConnector
}

func (e *emailAdapter) Send(ctx context.Context, to string, payload []byte) (int, error) {
	// Execute immediately; context cancellation isn't cleanly supported by net/smtp
	err := e.email.Send(to, "Graft Notification", payload)
	if err != nil {
		return 500, err
	}
	return 200, nil // Indicate success
}
