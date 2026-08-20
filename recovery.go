package multipart

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

type RecoveryRecord struct {
	UploadID   string
	Checkpoint int
	Attempts   int
	NextTry    time.Time
	LastError  string
}

type RecoveryBook struct {
	mu      sync.RWMutex
	records map[string]RecoveryRecord
}

func NewRecoveryBook() *RecoveryBook { return &RecoveryBook{records: make(map[string]RecoveryRecord)} }

func (b *RecoveryBook) Save(record RecoveryRecord) error {
	if record.UploadID == "" || record.Checkpoint < 0 || record.Attempts < 0 {
		return fmt.Errorf("invalid recovery record")
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	current, ok := b.records[record.UploadID]
	if ok && record.Checkpoint < current.Checkpoint {
		return fmt.Errorf("checkpoint regression for %s", record.UploadID)
	}
	b.records[record.UploadID] = record
	return nil
}

func (b *RecoveryBook) Load(uploadID string) (RecoveryRecord, bool) {
	b.mu.RLock()
	record, ok := b.records[uploadID]
	b.mu.RUnlock()
	return record, ok
}

func (b *RecoveryBook) RecordFailure(uploadID string, checkpoint int, err error, now time.Time) RecoveryRecord {
	b.mu.Lock()
	record := b.records[uploadID]
	record.UploadID = uploadID
	record.Checkpoint = checkpoint
	record.Attempts++
	record.NextTry = now.Add(backoffForAttempt(record.Attempts))
	if err != nil {
		record.LastError = err.Error()
	}
	b.records[uploadID] = record
	b.mu.Unlock()
	return record
}

func (b *RecoveryBook) Due(now time.Time, limit int) []RecoveryRecord {
	b.mu.RLock()
	items := make([]RecoveryRecord, 0, len(b.records))
	for _, record := range b.records {
		if record.NextTry.IsZero() || !record.NextTry.After(now) {
			items = append(items, record)
		}
	}
	b.mu.RUnlock()
	sort.Slice(items, func(i, j int) bool {
		if items[i].NextTry.Equal(items[j].NextTry) {
			return items[i].UploadID < items[j].UploadID
		}
		return items[i].NextTry.Before(items[j].NextTry)
	})
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return items
}

func (b *RecoveryBook) Delete(uploadID string) {
	b.mu.Lock()
	delete(b.records, uploadID)
	b.mu.Unlock()
}

func backoffForAttempt(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	if attempt > 8 {
		attempt = 8
	}
	return time.Second * time.Duration(1<<uint(attempt-1))
}
