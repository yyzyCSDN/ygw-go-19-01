package observability

import (
	"sort"
	"sync"
	"time"
)

type Outcome string

const (
	OutcomeAccepted Outcome = "accepted"
	OutcomeComplete Outcome = "complete"
	OutcomeFailed   Outcome = "failed"
	OutcomeRetried  Outcome = "retried"
)

type Sample struct {
	Tenant   string
	UploadID string
	Outcome  Outcome
	Bytes    int64
	Latency  time.Duration
	At       time.Time
}

type TenantMetrics struct {
	Tenant       string        `json:"tenant"`
	Accepted     int64         `json:"accepted"`
	Completed    int64         `json:"completed"`
	Failed       int64         `json:"failed"`
	Retried      int64         `json:"retried"`
	Bytes        int64         `json:"bytes"`
	TotalLatency time.Duration `json:"total_latency"`
	LastEventAt  time.Time     `json:"last_event_at"`
}

type Registry struct {
	mu      sync.RWMutex
	tenants map[string]TenantMetrics
	recent  []Sample
	limit   int
}

func NewRegistry(recentLimit int) *Registry {
	if recentLimit < 1 {
		recentLimit = 100
	}
	return &Registry{tenants: make(map[string]TenantMetrics), limit: recentLimit}
}

func (r *Registry) Record(sample Sample) {
	if sample.Tenant == "" || sample.UploadID == "" || sample.At.IsZero() {
		return
	}
	r.mu.Lock()
	metrics := r.tenants[sample.Tenant]
	metrics.Tenant = sample.Tenant
	switch sample.Outcome {
	case OutcomeAccepted:
		metrics.Accepted++
	case OutcomeComplete:
		metrics.Completed++
	case OutcomeFailed:
		metrics.Failed++
	case OutcomeRetried:
		metrics.Retried++
	}
	metrics.Bytes += sample.Bytes
	metrics.TotalLatency += sample.Latency
	if sample.At.After(metrics.LastEventAt) {
		metrics.LastEventAt = sample.At
	}
	r.tenants[sample.Tenant] = metrics
	r.recent = append(r.recent, sample)
	if len(r.recent) > r.limit {
		copy(r.recent, r.recent[len(r.recent)-r.limit:])
		r.recent = r.recent[:r.limit]
	}
	r.mu.Unlock()
}

func (r *Registry) Tenant(tenant string) (TenantMetrics, bool) {
	r.mu.RLock()
	metrics, ok := r.tenants[tenant]
	r.mu.RUnlock()
	return metrics, ok
}

func (r *Registry) All() []TenantMetrics {
	r.mu.RLock()
	result := make([]TenantMetrics, 0, len(r.tenants))
	for _, metrics := range r.tenants {
		result = append(result, metrics)
	}
	r.mu.RUnlock()
	sort.Slice(result, func(i, j int) bool { return result[i].Tenant < result[j].Tenant })
	return result
}

func (r *Registry) Recent(tenant string, since time.Time) []Sample {
	r.mu.RLock()
	result := make([]Sample, 0)
	for _, sample := range r.recent {
		if tenant != "" && sample.Tenant != tenant {
			continue
		}
		if !since.IsZero() && sample.At.Before(since) {
			continue
		}
		result = append(result, sample)
	}
	r.mu.RUnlock()
	return result
}

func (r *Registry) ErrorRate(tenant string) float64 {
	metrics, ok := r.Tenant(tenant)
	if !ok {
		return 0
	}
	total := metrics.Completed + metrics.Failed
	if total == 0 {
		return 0
	}
	return float64(metrics.Failed) / float64(total)
}

func (r *Registry) AverageLatency(tenant string) time.Duration {
	metrics, ok := r.Tenant(tenant)
	if !ok || metrics.Completed == 0 {
		return 0
	}
	return metrics.TotalLatency / time.Duration(metrics.Completed)
}
