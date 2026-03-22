package metrics

import (
	"encoding/json"
	"net/http"
	"sync/atomic"
)

var (
	webhooksReceived atomic.Uint64
	webhooksSuccess  atomic.Uint64
	webhooksFailed   atomic.Uint64
	forwardsTotal    atomic.Uint64
)

// IncWebhooksReceived records an accepted webhook after rule resolution.
func IncWebhooksReceived() { webhooksReceived.Add(1) }

// IncWebhooksSuccess records a completed forward.
func IncWebhooksSuccess() { webhooksSuccess.Add(1) }

// IncWebhooksFailed records a failed webhook pipeline step.
func IncWebhooksFailed() { webhooksFailed.Add(1) }

// AddForwards increments outbound HTTP attempt counter by n.
func AddForwards(n uint64) { forwardsTotal.Add(n) }

// ResetCountersForTest sets all counters to zero (tests only).
func ResetCountersForTest() {
	webhooksReceived.Store(0)
	webhooksSuccess.Store(0)
	webhooksFailed.Store(0)
	forwardsTotal.Store(0)
}

// Snapshot is a JSON-serializable metrics view.
type Snapshot struct {
	WebhooksReceived uint64 `json:"webhooks_received"`
	WebhooksSuccess  uint64 `json:"webhooks_success"`
	WebhooksFailed   uint64 `json:"webhooks_failed"`
	ForwardsTotal    uint64 `json:"forwards_total"`
}

// SnapshotNow returns current counter values.
func SnapshotNow() Snapshot {
	return Snapshot{
		WebhooksReceived: webhooksReceived.Load(),
		WebhooksSuccess:  webhooksSuccess.Load(),
		WebhooksFailed:   webhooksFailed.Load(),
		ForwardsTotal:    forwardsTotal.Load(),
	}
}

// WriteMetricsJSON writes metrics as JSON to w.
func WriteMetricsJSON(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(SnapshotNow())
}

