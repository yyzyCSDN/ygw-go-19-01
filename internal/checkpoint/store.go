package checkpoint

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"
)

type Stage string

const (
	StageAccepted  Stage = "accepted"
	StageInspect   Stage = "inspect"
	StageManifest  Stage = "manifest"
	StagePersisted Stage = "persisted"
)

type Record struct {
	UploadID string            `json:"upload_id"`
	Stage    Stage             `json:"stage"`
	Part     int               `json:"part"`
	Revision uint64            `json:"revision"`
	SavedAt  time.Time         `json:"saved_at"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

type Store interface {
	Load(context.Context, string) (Record, bool, error)
	Save(context.Context, Record) error
	Delete(context.Context, string) error
	List(context.Context) ([]Record, error)
}

type MemoryStore struct {
	mu      sync.RWMutex
	records map[string]Record
}

func NewMemoryStore() *MemoryStore { return &MemoryStore{records: make(map[string]Record)} }

func (s *MemoryStore) Load(ctx context.Context, uploadID string) (Record, bool, error) {
	if err := ctx.Err(); err != nil {
		return Record{}, false, err
	}
	s.mu.RLock()
	record, ok := s.records[uploadID]
	s.mu.RUnlock()
	return cloneRecord(record), ok, nil
}

func (s *MemoryStore) Save(ctx context.Context, record Record) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := validateRecord(record); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if current, ok := s.records[record.UploadID]; ok && record.Revision <= current.Revision {
		return fmt.Errorf("checkpoint revision %d is not newer than %d", record.Revision, current.Revision)
	}
	s.records[record.UploadID] = cloneRecord(record)
	return nil
}

func (s *MemoryStore) Delete(ctx context.Context, uploadID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	delete(s.records, uploadID)
	s.mu.Unlock()
	return nil
}

func (s *MemoryStore) List(ctx context.Context) ([]Record, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.RLock()
	result := make([]Record, 0, len(s.records))
	for _, record := range s.records {
		result = append(result, cloneRecord(record))
	}
	s.mu.RUnlock()
	sort.Slice(result, func(i, j int) bool { return result[i].UploadID < result[j].UploadID })
	return result, nil
}

func Encode(record Record) ([]byte, error) {
	if err := validateRecord(record); err != nil {
		return nil, err
	}
	data, err := json.Marshal(record)
	if err != nil {
		return nil, fmt.Errorf("encode checkpoint: %w", err)
	}
	return data, nil
}

func Decode(data []byte) (Record, error) {
	var record Record
	if err := json.Unmarshal(data, &record); err != nil {
		return Record{}, fmt.Errorf("decode checkpoint: %w", err)
	}
	if err := validateRecord(record); err != nil {
		return Record{}, err
	}
	return cloneRecord(record), nil
}

func validateRecord(record Record) error {
	if record.UploadID == "" || record.Revision == 0 || record.SavedAt.IsZero() {
		return fmt.Errorf("incomplete checkpoint")
	}
	switch record.Stage {
	case StageAccepted, StageInspect, StageManifest, StagePersisted:
		return nil
	default:
		return fmt.Errorf("unknown checkpoint stage %q", record.Stage)
	}
}

func cloneRecord(record Record) Record {
	out := record
	out.Metadata = make(map[string]string, len(record.Metadata))
	for key, value := range record.Metadata {
		out.Metadata[key] = value
	}
	return out
}
