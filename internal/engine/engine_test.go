package engine

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/DongDuong2001/graft/internal/models"
	"github.com/DongDuong2001/graft/internal/webhook"
)

func TestProcessFanOutAggregatesPartialFailure(t *testing.T) {
	repo := newEngineTestRepo(models.Rule{
		ID:         "rule-1",
		Name:       "fanout",
		ListenPath: "/hook/fanout",
		Destinations: []models.Destination{
			{URL: "https://first.example"},
			{URL: "https://second.example"},
		},
	})
	fwd := &fakeForwarder{
		results: map[string]fakeForwardResult{
			"https://first.example":  {status: 500, attempts: 2, err: errors.New("upstream failed")},
			"https://second.example": {status: 204, attempts: 1},
		},
	}
	eng := New(repo, "unused", fwd, nil)

	d, err := eng.Process(context.Background(), &webhook.Webhook{
		Path: "/hook/fanout",
		Body: []byte(`{"ok":true}`),
	})
	if err == nil {
		t.Fatal("expected partial fan-out error")
	}
	if d.Success {
		t.Fatal("partial failure must not be marked successful")
	}
	if d.Status != models.StatusPartial {
		t.Fatalf("status = %q, want %q", d.Status, models.StatusPartial)
	}
	if d.RetryCount != 1 {
		t.Fatalf("retry count = %d, want 1", d.RetryCount)
	}
	if !strings.Contains(d.ErrorMsg, "destination 1 failed") {
		t.Fatalf("error message = %q", d.ErrorMsg)
	}
}

func TestProcessFanOutSendsConcurrently(t *testing.T) {
	repo := newEngineTestRepo(models.Rule{
		ID:         "rule-1",
		Name:       "fanout",
		ListenPath: "/hook/fanout",
		Destinations: []models.Destination{
			{URL: "https://first.example"},
			{URL: "https://second.example"},
		},
	})
	fwd := &fakeForwarder{
		delay: 50 * time.Millisecond,
		results: map[string]fakeForwardResult{
			"https://first.example":  {status: 200, attempts: 1},
			"https://second.example": {status: 200, attempts: 1},
		},
	}
	eng := New(repo, "unused", fwd, nil)

	d, err := eng.Process(context.Background(), &webhook.Webhook{
		Path: "/hook/fanout",
		Body: []byte(`{"ok":true}`),
	})
	if err != nil {
		t.Fatalf("Process returned error: %v", err)
	}
	if !d.Success || d.Status != models.StatusDelivered {
		t.Fatalf("delivery = %+v", d)
	}
	if got := fwd.maxConcurrent.Load(); got < 2 {
		t.Fatalf("max concurrent sends = %d, want at least 2", got)
	}
}

func TestWorkerPoolStopContextLetsInFlightTaskFinish(t *testing.T) {
	repo := newEngineTestRepo(models.Rule{
		ID:             "rule-1",
		Name:           "worker",
		ListenPath:     "/hook/worker",
		DestinationURL: "https://worker.example",
	})
	started := make(chan struct{})
	fwd := &fakeForwarder{
		delay:   50 * time.Millisecond,
		started: started,
		results: map[string]fakeForwardResult{
			"https://worker.example": {status: 200, attempts: 1},
		},
	}
	eng := New(repo, "unused", fwd, nil)
	queue := NewMemoryQueue(1)
	pool := NewWorkerPool(queue, eng, 1)
	pool.Start(context.Background())

	err := queue.Enqueue(context.Background(), &Task{
		DeliveryID: "delivery-1",
		Webhook: &webhook.Webhook{
			Path: "/hook/worker",
			Body: []byte(`{"ok":true}`),
		},
		EnqueuedAt: time.Now(),
	})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for worker to start forwarding")
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := pool.StopContext(ctx); err != nil {
		t.Fatalf("StopContext: %v", err)
	}

	got, err := repo.GetDeliveryByID(context.Background(), "delivery-1")
	if err != nil {
		t.Fatalf("GetDeliveryByID: %v", err)
	}
	if got == nil || got.Status != models.StatusDelivered {
		t.Fatalf("delivery after stop = %+v", got)
	}
}

type fakeForwardResult struct {
	status   int
	attempts int
	err      error
}

type fakeForwarder struct {
	delay         time.Duration
	results       map[string]fakeForwardResult
	started       chan struct{}
	startedOnce   sync.Once
	inFlight      atomic.Int32
	maxConcurrent atomic.Int32
}

func (f *fakeForwarder) Send(ctx context.Context, rule *models.Rule, payload []byte) (int, int, error) {
	current := f.inFlight.Add(1)
	f.recordMaxConcurrent(current)
	defer f.inFlight.Add(-1)

	if f.started != nil {
		f.startedOnce.Do(func() {
			close(f.started)
		})
	}

	if f.delay > 0 {
		timer := time.NewTimer(f.delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return 0, 0, ctx.Err()
		case <-timer.C:
		}
	}

	result, ok := f.results[rule.DestinationURL]
	if !ok {
		return 200, 1, nil
	}
	return result.status, result.attempts, result.err
}

func (f *fakeForwarder) recordMaxConcurrent(current int32) {
	for {
		max := f.maxConcurrent.Load()
		if current <= max || f.maxConcurrent.CompareAndSwap(max, current) {
			return
		}
	}
}

type engineTestRepo struct {
	mu         sync.Mutex
	rule       models.Rule
	deliveries map[string]models.Delivery
}

func newEngineTestRepo(rule models.Rule) *engineTestRepo {
	return &engineTestRepo{
		rule:       rule,
		deliveries: make(map[string]models.Delivery),
	}
}

func (r *engineTestRepo) SaveRule(ctx context.Context, rule models.Rule) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rule = rule
	return nil
}

func (r *engineTestRepo) GetRuleByPath(ctx context.Context, path string) (*models.Rule, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.rule.ListenPath != path {
		return nil, nil
	}
	rule := r.rule
	return &rule, nil
}

func (r *engineTestRepo) GetRuleByID(ctx context.Context, id string) (*models.Rule, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.rule.ID != id {
		return nil, nil
	}
	rule := r.rule
	return &rule, nil
}

func (r *engineTestRepo) ListRules(ctx context.Context) ([]models.Rule, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return []models.Rule{r.rule}, nil
}

func (r *engineTestRepo) DeleteRule(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.rule.ID != id {
		return sql.ErrNoRows
	}
	r.rule = models.Rule{}
	return nil
}

func (r *engineTestRepo) SaveDelivery(ctx context.Context, d models.Delivery) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.deliveries[d.ID] = d
	return nil
}

func (r *engineTestRepo) GetDeliveryByID(ctx context.Context, id string) (*models.Delivery, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	d, ok := r.deliveries[id]
	if !ok {
		return nil, nil
	}
	return &d, nil
}

func (r *engineTestRepo) UpdateDeliveryStatus(ctx context.Context, id string, status string, errMsg string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	d := r.deliveries[id]
	d.ID = id
	d.Status = status
	d.ErrorMsg = errMsg
	r.deliveries[id] = d
	return nil
}

func (r *engineTestRepo) ListDeliveriesByRule(ctx context.Context, ruleID string, limit int) ([]models.Delivery, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []models.Delivery
	for _, d := range r.deliveries {
		if d.RuleID == ruleID {
			out = append(out, d)
		}
	}
	return out, nil
}
