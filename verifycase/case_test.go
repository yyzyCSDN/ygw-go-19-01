package verifycase

import (
	"context"
	"fmt"
	"testing"
	"time"

	"example.com/partflow/internal/scheduler"
)

func TestSchedulerDuePriorityOrder(t *testing.T) {
	now := time.Now().Truncate(time.Millisecond)
	clock := func() time.Time { return now }

	queue := scheduler.NewQueue()
	created := now
	future := scheduler.Job{ID: "future", Tenant: "t", UploadID: "u2", Priority: 5, Attempt: 0, NotBefore: now.Add(time.Hour), CreatedAt: created}
	due := scheduler.Job{ID: "due", Tenant: "t", UploadID: "u1", Priority: 1, Attempt: 0, NotBefore: now, CreatedAt: created}
	if err := queue.Enqueue(future); err != nil {
		t.Fatal(err)
	}
	if err := queue.Enqueue(due); err != nil {
		t.Fatal(err)
	}
	orderCtx, cancelOrder := context.WithCancel(context.Background())
	first, err := queue.Next(orderCtx, clock)
	cancelOrder()
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != "due" {
		t.Fatalf("first dequeued job=%s, want due", first.ID)
	}

	retryQueue := scheduler.NewQueue()
	started := make(chan struct{})
	attempts := 0
	handler := scheduler.HandlerFunc(func(_ context.Context, _ scheduler.Job) error {
		attempts++
		if attempts == 1 {
			close(started)
			return fmt.Errorf("boom")
		}
		return nil
	})
	pool, err := scheduler.NewWorkerPool(retryQueue, handler, 1, 3, clock)
	if err != nil {
		t.Fatal(err)
	}
	poolCtx, cancelPool := context.WithCancel(context.Background())
	defer cancelPool()
	if err := pool.Start(poolCtx); err != nil {
		t.Fatal(err)
	}
	retryJob := scheduler.Job{ID: "r1", Tenant: "t", UploadID: "u3", Priority: 1, Attempt: 0, NotBefore: now, CreatedAt: created}
	if err := retryQueue.Enqueue(retryJob); err != nil {
		t.Fatal(err)
	}
	<-started
	deadline := time.Now().Add(3 * time.Second)
	for retryQueue.Len() != 1 {
		if time.Now().After(deadline) {
			t.Fatal("retry job never re-enqueued")
		}
		time.Sleep(10 * time.Millisecond)
	}
	nextCtx, cancelNext := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancelNext()
	next, err := retryQueue.Next(nextCtx, func() time.Time { return now.Add(2 * time.Second) })
	if err != nil {
		t.Fatalf("retry job not due after backoff: %v", err)
	}
	if next.ID != "r1" || next.Attempt != 1 {
		t.Fatalf("retry job=%+v", next)
	}
	pool.Stop()
}
