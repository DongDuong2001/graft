package engine

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/DongDuong2001/graft/internal/webhook"
)

// Task represents a webhook processing unit in the queue.
type Task struct {
	DeliveryID string
	Webhook    *webhook.Webhook
	EnqueuedAt time.Time
}

// Queue defines how tasks are stored and retrieved.
type Queue interface {
	Enqueue(ctx context.Context, task *Task) error
	Dequeue(ctx context.Context) (*Task, error)
	Size() int
}

// MemoryQueue is a simple channel-based in-memory queue.
type MemoryQueue struct {
	ch chan *Task
}

func NewMemoryQueue(size int) *MemoryQueue {
	return &MemoryQueue{
		ch: make(chan *Task, size),
	}
}

func (q *MemoryQueue) Enqueue(ctx context.Context, task *Task) error {
	select {
	case q.ch <- task:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		return context.DeadlineExceeded // Full queue
	}
}

func (q *MemoryQueue) Dequeue(ctx context.Context) (*Task, error) {
	select {
	case t := <-q.ch:
		return t, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (q *MemoryQueue) Size() int {
	return len(q.ch)
}

// WorkerPool manages background processing of tasks.
type WorkerPool struct {
	queue  Queue
	engine *Engine
	count  int
	wg     sync.WaitGroup
	cancel context.CancelFunc
	once   sync.Once
}

func NewWorkerPool(q Queue, eng *Engine, count int) *WorkerPool {
	if count <= 0 {
		count = 4 // Default
	}
	return &WorkerPool{
		queue:  q,
		engine: eng,
		count:  count,
	}
}

func (p *WorkerPool) Start(ctx context.Context) {
	ctx, p.cancel = context.WithCancel(ctx)
	for i := 0; i < p.count; i++ {
		p.wg.Add(1)
		go p.worker(ctx, i)
	}
	slog.Info("Worker pool started", "count", p.count)
}

func (p *WorkerPool) Stop() {
	if err := p.StopContext(context.Background()); err != nil {
		slog.Error("Worker pool stop failed", "error", err)
	}
}

func (p *WorkerPool) StopContext(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	p.once.Do(func() {
		if p.cancel != nil {
			p.cancel()
		}
	})

	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		slog.Info("Worker pool stopped")
		return nil
	case <-ctx.Done():
		return fmt.Errorf("worker pool shutdown: %w", ctx.Err())
	}
}

func (p *WorkerPool) worker(ctx context.Context, id int) {
	defer p.wg.Done()
	for {
		task, err := p.queue.Dequeue(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			slog.Error("Failed to dequeue task", "worker_id", id, "error", err)
			continue
		}

		slog.Debug("Processing task", "worker_id", id, "delivery_id", task.DeliveryID)
		processCtx := context.WithoutCancel(ctx)
		err = p.engine.ProcessAsync(processCtx, task.DeliveryID, task.Webhook)
		if err != nil {
			slog.Error("Task processing failed", "worker_id", id, "delivery_id", task.DeliveryID, "error", err)
		}
	}
}
