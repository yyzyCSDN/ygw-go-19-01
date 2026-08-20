package scheduler

import (
	"container/heap"
	"context"
	"fmt"
	"sync"
	"time"
)

type Job struct {
	ID        string
	Tenant    string
	UploadID  string
	Priority  int
	Attempt   int
	NotBefore time.Time
	CreatedAt time.Time
}

type queuedJob struct {
	job   Job
	index int
}

type jobHeap []*queuedJob

func (h jobHeap) Len() int { return len(h) }
func (h jobHeap) Less(i, j int) bool {
	if !h[i].job.NotBefore.Equal(h[j].job.NotBefore) {
		return h[i].job.NotBefore.Before(h[j].job.NotBefore)
	}
	if h[i].job.Priority != h[j].job.Priority {
		return h[i].job.Priority > h[j].job.Priority
	}
	return h[i].job.CreatedAt.Before(h[j].job.CreatedAt)
}
func (h jobHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i]; h[i].index = i; h[j].index = j }
func (h *jobHeap) Push(value any) {
	item := value.(*queuedJob)
	item.index = len(*h)
	*h = append(*h, item)
}
func (h *jobHeap) Pop() any {
	old := *h
	item := old[len(old)-1]
	item.index = -1
	*h = old[:len(old)-1]
	return item
}

type Queue struct {
	mu     sync.Mutex
	jobs   jobHeap
	byID   map[string]*queuedJob
	wake   chan struct{}
	closed bool
}

func NewQueue() *Queue {
	queue := &Queue{byID: make(map[string]*queuedJob), wake: make(chan struct{})}
	heap.Init(&queue.jobs)
	return queue
}

func (q *Queue) Enqueue(job Job) error {
	if job.ID == "" || job.Tenant == "" || job.UploadID == "" || job.CreatedAt.IsZero() {
		return fmt.Errorf("incomplete scheduler job")
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		return fmt.Errorf("scheduler queue is closed")
	}
	if existing, ok := q.byID[job.ID]; ok {
		// Enforce a strict monotonic gate per job ID: a queued attempt may
		// only be superseded by a strictly greater one. A replay of the
		// same attempt or a regression to an older one is rejected
		// explicitly so a duplicate or stale delivery can never replace or
		// duplicate the live attempt. Normal retries always carry attempt+1,
		// so they pass through untouched. The gate and the replacement run
		// under the same lock, so two concurrent enqueues cannot both observe
		// a stale attempt.
		switch {
		case job.Attempt < existing.job.Attempt:
			return fmt.Errorf("%w: %s attempt %d < %d", ErrAttemptRegression, job.ID, job.Attempt, existing.job.Attempt)
		case job.Attempt == existing.job.Attempt:
			return fmt.Errorf("%w: %s attempt %d already queued", ErrAttemptConflict, job.ID, job.Attempt)
		}
		existing.job = job
		heap.Fix(&q.jobs, existing.index)
		q.signal()
		return nil
	}
	item := &queuedJob{job: job}
	heap.Push(&q.jobs, item)
	q.byID[job.ID] = item
	q.signal()
	return nil
}

func (q *Queue) Next(ctx context.Context, now func() time.Time) (Job, error) {
	if now == nil {
		now = time.Now
	}
	for {
		q.mu.Lock()
		if q.closed {
			q.mu.Unlock()
			return Job{}, fmt.Errorf("scheduler queue is closed")
		}
		if len(q.jobs) > 0 {
			item := q.jobs[0]
			wait := item.job.NotBefore.Sub(now())
			if wait <= 0 {
				heap.Pop(&q.jobs)
				delete(q.byID, item.job.ID)
				q.mu.Unlock()
				return item.job, nil
			}
			wake := q.wake
			q.mu.Unlock()
			timer := time.NewTimer(wait)
			select {
			case <-ctx.Done():
				timer.Stop()
				return Job{}, ctx.Err()
			case <-wake:
				timer.Stop()
			case <-timer.C:
			}
			continue
		}
		wake := q.wake
		q.mu.Unlock()
		select {
		case <-ctx.Done():
			return Job{}, ctx.Err()
		case <-wake:
		}
	}
}

func (q *Queue) Remove(jobID string) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	item, ok := q.byID[jobID]
	if !ok {
		return false
	}
	heap.Remove(&q.jobs, item.index)
	delete(q.byID, jobID)
	q.signal()
	return true
}

func (q *Queue) Len() int { q.mu.Lock(); defer q.mu.Unlock(); return len(q.jobs) }

func (q *Queue) Close() {
	q.mu.Lock()
	if !q.closed {
		q.closed = true
		q.signal()
	}
	q.mu.Unlock()
}

func (q *Queue) signal() { close(q.wake); q.wake = make(chan struct{}) }
