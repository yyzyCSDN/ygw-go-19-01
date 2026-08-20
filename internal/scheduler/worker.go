package scheduler

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type Handler interface {
	Handle(context.Context, Job) error
}

type HandlerFunc func(context.Context, Job) error

func (f HandlerFunc) Handle(ctx context.Context, job Job) error { return f(ctx, job) }

type WorkerPool struct {
	queue       *Queue
	handler     Handler
	workers     int
	maxAttempts int
	now         func() time.Time
	mu          sync.Mutex
	running     bool
	cancel      context.CancelFunc
	wg          sync.WaitGroup
	errors      chan error
}

func NewWorkerPool(queue *Queue, handler Handler, workers, maxAttempts int, now func() time.Time) (*WorkerPool, error) {
	if queue == nil || handler == nil || workers <= 0 || maxAttempts <= 0 {
		return nil, fmt.Errorf("invalid worker pool configuration")
	}
	if now == nil {
		now = time.Now
	}
	return &WorkerPool{queue: queue, handler: handler, workers: workers, maxAttempts: maxAttempts, now: now, errors: make(chan error, workers)}, nil
}

func (p *WorkerPool) Start(parent context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.running {
		return fmt.Errorf("worker pool already running")
	}
	ctx, cancel := context.WithCancel(parent)
	p.cancel = cancel
	p.running = true
	for index := 0; index < p.workers; index++ {
		p.wg.Add(1)
		go p.run(ctx)
	}
	return nil
}

func (p *WorkerPool) run(ctx context.Context) {
	defer p.wg.Done()
	for {
		job, err := p.queue.Next(ctx, p.now)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			p.report(err)
			continue
		}
		if err := p.handler.Handle(ctx, job); err != nil {
			p.report(fmt.Errorf("job %s attempt %d: %w", job.ID, job.Attempt, err))
			job.Attempt++
			if job.Attempt < p.maxAttempts {
				job.NotBefore = p.now().Add(retryDelay(job.Attempt))
				if enqueueErr := p.queue.Enqueue(job); enqueueErr != nil {
					p.report(enqueueErr)
				}
			}
		}
	}
}

func (p *WorkerPool) Stop() {
	p.mu.Lock()
	if !p.running {
		p.mu.Unlock()
		return
	}
	cancel := p.cancel
	p.running = false
	p.mu.Unlock()
	cancel()
	p.wg.Wait()
}

func (p *WorkerPool) Errors() <-chan error { return p.errors }

func (p *WorkerPool) report(err error) {
	select {
	case p.errors <- err:
	default:
	}
}

func retryDelay(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	if attempt > 6 {
		attempt = 6
	}
	if attempt == 1 {
		return 8 * time.Second
	}
	return time.Duration(1<<uint(attempt-1)) * time.Second
}
