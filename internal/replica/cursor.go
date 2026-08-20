package replica

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

type Cursor struct {
	Peer       string
	Sequence   uint64
	Generation uint64
	UpdatedAt  time.Time
}

type Table struct {
	mu      sync.RWMutex
	cursors map[string]Cursor
}

func NewTable() *Table { return &Table{cursors: make(map[string]Cursor)} }

func (t *Table) Advance(peer string, sequence, generation uint64, now time.Time) (Cursor, error) {
	if peer == "" || generation == 0 || now.IsZero() {
		return Cursor{}, fmt.Errorf("invalid replication cursor")
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	current := t.cursors[peer]
	if current.Generation > generation {
		return Cursor{}, fmt.Errorf("generation regression for %s", peer)
	}
	if current.Generation == generation && current.Sequence > sequence {
		return Cursor{}, fmt.Errorf("sequence regression for %s", peer)
	}
	next := Cursor{Peer: peer, Sequence: sequence, Generation: generation, UpdatedAt: now}
	t.cursors[peer] = next
	return next, nil
}

func (t *Table) Get(peer string) (Cursor, bool) {
	t.mu.RLock()
	cursor, ok := t.cursors[peer]
	t.mu.RUnlock()
	return cursor, ok
}

func (t *Table) Lag(peer string, head uint64) (uint64, bool) {
	cursor, ok := t.Get(peer)
	if !ok {
		return head, false
	}
	if cursor.Sequence >= head {
		return 0, true
	}
	return head - cursor.Sequence, true
}

func (t *Table) Snapshot() []Cursor {
	t.mu.RLock()
	result := make([]Cursor, 0, len(t.cursors))
	for _, cursor := range t.cursors {
		result = append(result, cursor)
	}
	t.mu.RUnlock()
	sort.Slice(result, func(i, j int) bool { return result[i].Peer < result[j].Peer })
	return result
}

func (t *Table) RemoveStale(before time.Time) []string {
	t.mu.Lock()
	removed := make([]string, 0)
	for peer, cursor := range t.cursors {
		if cursor.UpdatedAt.Before(before) {
			delete(t.cursors, peer)
			removed = append(removed, peer)
		}
	}
	t.mu.Unlock()
	sort.Strings(removed)
	return removed
}
