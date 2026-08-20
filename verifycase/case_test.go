package verifycase

import (
	"context"
	"fmt"
	"testing"
	"time"

	"example.com/partflow/internal/journal"
	"example.com/partflow/internal/replica"
)

type failingSink struct{}

func (failingSink) Apply(context.Context, replica.Batch) error { return fmt.Errorf("apply failed") }

func TestTenantCursorIsolation(t *testing.T) {
	log := journal.NewLog()
	now := time.Now().Truncate(time.Millisecond)
	appendEvent := func(tenant, id string, kind journal.Kind) {
		if _, err := log.Append(journal.Event{Tenant: tenant, UploadID: id, Kind: kind, At: now}); err != nil {
			t.Fatal(err)
		}
	}
	appendEvent("a", "u1", journal.UploadOpened)
	appendEvent("b", "u2", journal.UploadOpened)
	appendEvent("a", "u3", journal.PartAccepted)

	items, cursor := log.Read(journal.Cursor{Tenant: "a"}, 0)
	if len(items) != 2 {
		t.Fatalf("tenant a events=%d, want 2", len(items))
	}
	for _, event := range items {
		if event.Tenant != "a" {
			t.Fatalf("tenant cursor leaked event %+v", event)
		}
	}
	if cursor.Sequence != 3 {
		t.Fatalf("cursor sequence=%d, want 3", cursor.Sequence)
	}

	otherLog := journal.NewLog()
	if _, err := otherLog.Append(journal.Event{Tenant: "b", UploadID: "u9", Kind: journal.UploadOpened, At: now}); err != nil {
		t.Fatal(err)
	}
	waitCtx, cancelWait := context.WithTimeout(context.Background(), 300*time.Millisecond)
	_, _, waitErr := otherLog.Wait(waitCtx, journal.Cursor{Tenant: "a"})
	cancelWait()
	if waitErr == nil {
		t.Fatal("Wait returned for a cursor with no matching tenant events")
	}

	table := replica.NewTable()
	replicator, err := replica.NewReplicator(table, failingSink{}, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	batch := replica.Batch{
		Generation: 1,
		From:       1,
		To:         1,
		Entries:    []replica.Entry{{Sequence: 1, UploadID: "u1", Payload: []byte("p"), Digest: "d"}},
	}
	if err := replicator.Send(context.Background(), "peer", batch); err == nil {
		t.Fatal("Send with failing sink succeeded")
	}
	if cursor, ok := table.Get("peer"); ok && cursor.Sequence != 0 {
		t.Fatalf("peer cursor advanced after failed apply: %+v", cursor)
	}
}
