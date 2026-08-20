package scheduler

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func newJob(id string, attempt int) Job {
	return Job{
		ID:        id,
		Tenant:    "demo",
		UploadID:  "upload-" + id,
		Priority:  1,
		Attempt:   attempt,
		CreatedAt: time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC),
	}
}

func TestEnqueueAcceptsStrictlyGreaterAttempt(t *testing.T) {
	queue := NewQueue()
	defer queue.Close()

	if err := queue.Enqueue(newJob("j", 1)); err != nil {
		t.Fatalf("enqueue attempt 1: %v", err)
	}
	if err := queue.Enqueue(newJob("j", 3)); err != nil {
		t.Fatalf("enqueue attempt 3: %v", err)
	}
	if got := queue.Len(); got != 1 {
		t.Fatalf("len=%d want 1 (replace leaked a duplicate)", got)
	}
}

func TestEnqueueRejectsEqualAttempt(t *testing.T) {
	queue := NewQueue()
	defer queue.Close()

	if err := queue.Enqueue(newJob("j", 2)); err != nil {
		t.Fatalf("enqueue attempt 2: %v", err)
	}
	err := queue.Enqueue(newJob("j", 2))
	if !errors.Is(err, ErrAttemptConflict) {
		t.Fatalf("err=%v want ErrAttemptConflict", err)
	}
	if got := queue.Len(); got != 1 {
		t.Fatalf("len=%d want 1 (equal silently replaced/duplicated)", got)
	}
}

func TestEnqueueRejectsRegression(t *testing.T) {
	queue := NewQueue()
	defer queue.Close()

	if err := queue.Enqueue(newJob("j", 5)); err != nil {
		t.Fatalf("enqueue attempt 5: %v", err)
	}
	err := queue.Enqueue(newJob("j", 4))
	if !errors.Is(err, ErrAttemptRegression) {
		t.Fatalf("err=%v want ErrAttemptRegression", err)
	}
	if got := queue.Len(); got != 1 {
		t.Fatalf("len=%d want 1 (regression revived a stale attempt)", got)
	}
}

// TestRetryRhythmPreserved models the worker's normal retry path: a job is
// dequeued (and thus removed from the index), then re-enqueued with attempt+1.
// That must succeed every time — the monotonic gate must never block a
// legitimate retry.
func TestRetryRhythmPreserved(t *testing.T) {
	queue := NewQueue()
	defer queue.Close()

	now := time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)
	if err := queue.Enqueue(newJob("j", 1)); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	for attempt := 1; attempt <= 3; attempt++ {
		// Next pops the job and removes it from byID, exactly as the worker
		// does; advance the clock past the backoff window so it is ready.
		now = now.Add(time.Hour)
		job, err := queue.Next(context.Background(), func() time.Time { return now })
		if err != nil {
			t.Fatalf("next attempt %d: %v", attempt, err)
		}
		if job.Attempt != attempt {
			t.Fatalf("popped attempt %d want %d", job.Attempt, attempt)
		}
		// Simulate a failure: bump the attempt and re-enqueue. This hits the
		// "no existing entry" path because the pop removed it, so it must
		// always succeed.
		next := job
		next.Attempt++
		if err := queue.Enqueue(next); err != nil {
			t.Fatalf("re-enqueue attempt %d: %v", next.Attempt, err)
		}
	}
}

// TestEnqueueMonotonicUnderConcurrency asserts that concurrent re-enqueues of
// the same job ID can never leave the queue holding an older attempt than the
// maximum submitted, and never leaves duplicate heap entries.
func TestEnqueueMonotonicUnderConcurrency(t *testing.T) {
	queue := NewQueue()
	defer queue.Close()

	if err := queue.Enqueue(newJob("shared", 0)); err != nil {
		t.Fatalf("seed: %v", err)
	}

	const workers = 32
	maxAttempt := 0
	var mu sync.Mutex

	var wg sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		wg.Add(1)
		go func(base int) {
			defer wg.Done()
			for attempt := 1; attempt <= 64; attempt++ {
				value := base*64 + attempt
				err := queue.Enqueue(newJob("shared", value))
				if err == nil {
					mu.Lock()
					if value > maxAttempt {
						maxAttempt = value
					}
					mu.Unlock()
				}
			}
		}(worker)
	}
	wg.Wait()

	if got := queue.Len(); got != 1 {
		t.Fatalf("len=%d want 1 (gate leaked duplicate entries)", got)
	}

	// Drain the single entry and confirm it carries the maximum accepted
	// attempt, never an older one.
	now := time.Date(2026, 8, 19, 13, 0, 0, 0, time.UTC)
	job, err := queue.Next(context.Background(), func() time.Time { return now })
	if err != nil {
		t.Fatalf("next: %v", err)
	}
	if job.Attempt != maxAttempt {
		t.Fatalf("attempt=%d want %d (a smaller attempt clobbered the max)", job.Attempt, maxAttempt)
	}
}
