package observability

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestResetCountersForTest(t *testing.T) {
	ResetCountersForTest()
	IncWebhooksReceived()
	IncWebhooksSuccess()
	IncWebhooksFailed()
	AddForwards(2)

	s := SnapshotNow()
	if s.WebhooksReceived != 1 || s.WebhooksSuccess != 1 || s.WebhooksFailed != 1 || s.ForwardsTotal != 2 {
		t.Fatalf("%+v", s)
	}

	ResetCountersForTest()
	s = SnapshotNow()
	if s.WebhooksReceived != 0 || s.WebhooksSuccess != 0 || s.WebhooksFailed != 0 || s.ForwardsTotal != 0 {
		t.Fatalf("after reset: %+v", s)
	}
}

func TestWriteMetricsJSON(t *testing.T) {
	ResetCountersForTest()
	IncWebhooksReceived()
	rec := httptest.NewRecorder()
	WriteMetricsJSON(rec)
	if rec.Code != http.StatusOK {
		t.Fatalf("code %d", rec.Code)
	}
	var m map[string]uint64
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatal(err)
	}
	if m["webhooks_received"] != 1 {
		t.Fatalf("%v", m)
	}
}
