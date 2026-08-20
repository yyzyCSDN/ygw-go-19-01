package replica

import (
	"context"
	"errors"
	"testing"
	"time"
)

func fixedTime() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) }

type stubSink struct {
	fail  error
	calls []Batch
}

func (s *stubSink) Apply(_ context.Context, batch Batch) error {
	s.calls = append(s.calls, batch)
	return s.fail
}

func singleBatch(gen, seq uint64) Batch {
	return Batch{
		Generation: gen,
		From:       seq,
		To:         seq,
		Entries:    []Entry{{Sequence: seq, UploadID: "u", Payload: []byte("p"), Digest: "d"}},
	}
}

func TestSendAdvancesCursorWhenApplySucceeds(t *testing.T) {
	table := NewTable()
	sink := &stubSink{}
	replicator, err := NewReplicator(table, sink, fixedTime)
	if err != nil {
		t.Fatal(err)
	}

	if err := replicator.Send(context.Background(), "peer-a", singleBatch(1, 1)); err != nil {
		t.Fatalf("send: %v", err)
	}
	cursor, ok := table.Get("peer-a")
	if !ok {
		t.Fatal("peer cursor not recorded after successful apply")
	}
	if cursor.Sequence != 1 || cursor.Generation != 1 {
		t.Fatalf("peer cursor = %+v, want seq=1 gen=1", cursor)
	}
	if len(sink.calls) != 1 {
		t.Fatalf("sink applied %d times, want 1", len(sink.calls))
	}
}

func TestSendDoesNotAdvanceCursorWhenApplyFails(t *testing.T) {
	table := NewTable()
	sink := &stubSink{fail: errors.New("sink down")}
	replicator, err := NewReplicator(table, sink, fixedTime)
	if err != nil {
		t.Fatal(err)
	}

	err = replicator.Send(context.Background(), "peer-a", singleBatch(1, 1))
	if err == nil {
		t.Fatal("expected send to fail when apply fails")
	}
	if cursor, ok := table.Get("peer-a"); ok {
		t.Fatalf("peer cursor advanced to %+v despite apply failure", cursor)
	}
}

func TestSendDoesNotAdvanceCursorOnInvalidBatch(t *testing.T) {
	table := NewTable()
	sink := &stubSink{}
	replicator, _ := NewReplicator(table, sink, fixedTime)

	// From == 0 fails validateBatch without touching the sink or cursor.
	bad := Batch{Generation: 1, From: 0, To: 0, Entries: []Entry{{Sequence: 0, UploadID: "u", Payload: []byte("p"), Digest: "d"}}}
	if err := replicator.Send(context.Background(), "peer-a", bad); err == nil {
		t.Fatal("expected invalid batch error")
	}
	if len(sink.calls) != 0 {
		t.Fatalf("sink applied invalid batch: %+v", sink.calls)
	}
	if _, ok := table.Get("peer-a"); ok {
		t.Fatal("peer cursor advanced for invalid batch")
	}
}
