package forwarder

import (
	"context"

	"Graft/internal/connectors"
	"Graft/internal/models"
)

// Forwarder defines the interface for sending webhooks to destinations.
type Forwarder interface {
	Send(ctx context.Context, rule *models.Rule, payload []byte) (statusCode int, attempts int, err error)
}

// SyncForwarder implements Forwarder using synchronous HTTP calls.
type SyncForwarder struct {
	connector *connectors.HTTPForwarder
}

// NewSyncForwarder creates a new forwarder with the given configuration.
func NewSyncForwarder(client *connectors.HTTPForwarder) *SyncForwarder {
	return &SyncForwarder{
		connector: client,
	}
}

// Send delegates to the underlying connector.
func (f *SyncForwarder) Send(ctx context.Context, rule *models.Rule, payload []byte) (statusCode int, attempts int, err error) {
	return f.connector.Send(ctx, rule, payload)
}
