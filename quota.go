package multipart

import (
	"fmt"
	"sync"
	"time"
)

type QuotaPolicy struct {
	MaxOpenUploads int
	MaxBytes       int64
	Window         time.Duration
}

type QuotaUsage struct {
	Tenant      string
	OpenUploads int
	Bytes       int64
	WindowStart time.Time
}

type QuotaLedger struct {
	mu     sync.Mutex
	policy map[string]QuotaPolicy
	usage  map[string]QuotaUsage
	clock  func() time.Time
}

func NewQuotaLedger(clock func() time.Time) *QuotaLedger {
	if clock == nil {
		clock = time.Now
	}
	return &QuotaLedger{policy: make(map[string]QuotaPolicy), usage: make(map[string]QuotaUsage), clock: clock}
}

func (l *QuotaLedger) SetPolicy(tenant string, policy QuotaPolicy) error {
	if tenant == "" || policy.MaxOpenUploads <= 0 || policy.MaxBytes <= 0 || policy.Window <= 0 {
		return fmt.Errorf("invalid quota policy")
	}
	l.mu.Lock()
	l.policy[tenant] = policy
	l.mu.Unlock()
	return nil
}

func (l *QuotaLedger) Reserve(tenant string, bytes int64) (func(bool), error) {
	if bytes < 0 {
		return nil, fmt.Errorf("negative reservation")
	}
	l.mu.Lock()
	policy, ok := l.policy[tenant]
	if !ok {
		l.mu.Unlock()
		return nil, fmt.Errorf("quota policy missing for %s", tenant)
	}
	now := l.clock()
	usage := l.usage[tenant]
	if usage.WindowStart.IsZero() || !usage.WindowStart.Add(policy.Window).After(now) {
		usage = QuotaUsage{Tenant: tenant, WindowStart: now}
	}
	if usage.OpenUploads+1 > policy.MaxOpenUploads || usage.Bytes+bytes > policy.MaxBytes {
		l.mu.Unlock()
		return nil, fmt.Errorf("quota exceeded for %s", tenant)
	}
	usage.OpenUploads++
	usage.Bytes += bytes
	l.usage[tenant] = usage
	l.mu.Unlock()
	var once sync.Once
	return func(commit bool) {
		once.Do(func() {
			l.mu.Lock()
			current := l.usage[tenant]
			if current.OpenUploads > 0 {
				current.OpenUploads--
			}
			if !commit && current.Bytes >= bytes {
				current.Bytes -= bytes
			}
			l.usage[tenant] = current
			l.mu.Unlock()
		})
	}, nil
}

func (l *QuotaLedger) Usage(tenant string) QuotaUsage {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.usage[tenant]
}
