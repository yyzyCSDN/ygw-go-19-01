package replica

import (
	"context"
	"fmt"
	"time"
)

type Entry struct {
	Sequence uint64
	UploadID string
	Payload  []byte
	Digest   string
}

type Batch struct {
	Generation uint64
	From       uint64
	To         uint64
	Entries    []Entry
}

type Sink interface {
	Apply(context.Context, Batch) error
}

type Replicator struct {
	table *Table
	sink  Sink
	now   func() time.Time
}

func NewReplicator(table *Table, sink Sink, now func() time.Time) (*Replicator, error) {
	if table == nil || sink == nil {
		return nil, fmt.Errorf("replicator dependencies are required")
	}
	if now == nil {
		now = time.Now
	}
	return &Replicator{table: table, sink: sink, now: now}, nil
}

func (r *Replicator) Send(ctx context.Context, peer string, batch Batch) error {
	if peer == "" || batch.Generation == 0 || len(batch.Entries) == 0 {
		return fmt.Errorf("invalid replication batch")
	}
	if err := validateBatch(batch); err != nil {
		return err
	}
	if err := r.sink.Apply(ctx, cloneBatch(batch)); err != nil {
		return fmt.Errorf("apply replication batch: %w", err)
	}
	// The peer cursor records applied progress, so it may only advance once
	// the batch has been applied by the sink. Advancing before Apply would
	// mark a failed batch as consumed and drop its events on the next retry.
	_, err := r.table.Advance(peer, batch.To, batch.Generation, r.now())
	return err
}

func validateBatch(batch Batch) error {
	if batch.From == 0 || batch.To < batch.From {
		return fmt.Errorf("invalid replication range")
	}
	for index, entry := range batch.Entries {
		expected := batch.From + uint64(index)
		if entry.Sequence != expected {
			return fmt.Errorf("invalid replication entry at sequence %d", expected)
		}
	}
	return nil
}

func cloneBatch(batch Batch) Batch {
	out := batch
	out.Entries = make([]Entry, len(batch.Entries))
	for index, entry := range batch.Entries {
		entry.Payload = append([]byte(nil), entry.Payload...)
		out.Entries[index] = entry
	}
	return out
}
