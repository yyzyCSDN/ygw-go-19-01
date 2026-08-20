package multipart

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

type Lease struct {
	UploadID string
	Owner    string
	Token    uint64
	Until    time.Time
}

type LeaseBook struct {
	mu     sync.Mutex
	leases map[string]Lease
	next   uint64
}

func NewLeaseBook() *LeaseBook { return &LeaseBook{leases: make(map[string]Lease)} }

func (b *LeaseBook) Acquire(uploadID, owner string, ttl time.Duration, now time.Time) (Lease, error) {
	if uploadID == "" || owner == "" || ttl <= 0 {
		return Lease{}, fmt.Errorf("invalid lease request")
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if current, ok := b.leases[uploadID]; ok && current.Until.After(now) && current.Owner != owner {
		return Lease{}, fmt.Errorf("upload %s leased by %s", uploadID, current.Owner)
	}
	b.next++
	lease := Lease{UploadID: uploadID, Owner: owner, Token: b.next, Until: now.Add(ttl)}
	b.leases[uploadID] = lease
	return lease, nil
}

func (b *LeaseBook) Renew(lease Lease, ttl time.Duration, now time.Time) (Lease, error) {
	if ttl <= 0 {
		return Lease{}, fmt.Errorf("invalid lease ttl")
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	current, ok := b.leases[lease.UploadID]
	if !ok || current.Token != lease.Token || current.Owner != lease.Owner {
		return Lease{}, fmt.Errorf("stale lease for %s", lease.UploadID)
	}
	if !current.Until.After(now) {
		delete(b.leases, lease.UploadID)
		return Lease{}, fmt.Errorf("lease for %s expired", lease.UploadID)
	}
	current.Until = now.Add(ttl)
	b.leases[lease.UploadID] = current
	return current, nil
}

func (b *LeaseBook) Release(lease Lease) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	current, ok := b.leases[lease.UploadID]
	if !ok || current.Token != lease.Token || current.Owner != lease.Owner {
		return false
	}
	delete(b.leases, lease.UploadID)
	return true
}

func (b *LeaseBook) Snapshot(now time.Time) []Lease {
	b.mu.Lock()
	defer b.mu.Unlock()
	result := make([]Lease, 0, len(b.leases))
	for id, lease := range b.leases {
		if !lease.Until.After(now) {
			delete(b.leases, id)
			continue
		}
		result = append(result, lease)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].UploadID < result[j].UploadID })
	return result
}
