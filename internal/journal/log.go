package journal

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"
)

type Kind string

const (
	UploadOpened    Kind = "upload.opened"
	PartAccepted    Kind = "part.accepted"
	InspectionDone  Kind = "inspection.done"
	ManifestWritten Kind = "manifest.written"
	UploadFailed    Kind = "upload.failed"
)

type Event struct {
	Sequence uint64
	Tenant   string
	UploadID string
	Kind     Kind
	At       time.Time
	Fields   map[string]string
}

type Cursor struct {
	Tenant   string
	Sequence uint64
}

type Log struct {
	mu     sync.RWMutex
	next   uint64
	events []Event
	wake   chan struct{}
}

func NewLog() *Log { return &Log{wake: make(chan struct{})} }

func (l *Log) Append(event Event) (Event, error) {
	if event.Tenant == "" || event.UploadID == "" || event.Kind == "" || event.At.IsZero() {
		return Event{}, fmt.Errorf("incomplete journal event")
	}
	l.mu.Lock()
	l.next++
	event.Sequence = l.next
	event.Fields = cloneFields(event.Fields)
	l.events = append(l.events, event)
	close(l.wake)
	l.wake = make(chan struct{})
	l.mu.Unlock()
	return cloneEvent(event), nil
}

func (l *Log) Read(cursor Cursor, limit int) ([]Event, Cursor) {
	l.mu.RLock()
	result := make([]Event, 0)
	last := cursor.Sequence
	for _, event := range l.events {
		if event.Sequence <= cursor.Sequence || (cursor.Tenant != "" && event.Tenant != cursor.Tenant) {
			continue
		}
		result = append(result, cloneEvent(event))
		last = event.Sequence
		if limit > 0 && len(result) == limit {
			break
		}
	}
	l.mu.RUnlock()
	return result, Cursor{Tenant: cursor.Tenant, Sequence: last}
}

func (l *Log) Wait(ctx context.Context, cursor Cursor) ([]Event, Cursor, error) {
	for {
		items, next := l.Read(cursor, 0)
		if len(items) > 0 {
			return items, next, nil
		}
		l.mu.RLock()
		wake := l.wake
		l.mu.RUnlock()
		select {
		case <-ctx.Done():
			return nil, cursor, ctx.Err()
		case <-wake:
		}
	}
}

func (l *Log) Compact(before uint64) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	index := sort.Search(len(l.events), func(i int) bool { return l.events[i].Sequence >= before })
	if index == 0 {
		return 0
	}
	l.events = append([]Event(nil), l.events[index:]...)
	return index
}

func (l *Log) Size() int {
	l.mu.RLock()
	size := len(l.events)
	l.mu.RUnlock()
	return size
}

func cloneEvent(event Event) Event {
	event.Fields = cloneFields(event.Fields)
	return event
}

func cloneFields(fields map[string]string) map[string]string {
	out := make(map[string]string, len(fields))
	for key, value := range fields {
		out[key] = value
	}
	return out
}
