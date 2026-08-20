package journal

import (
	"context"
	"testing"
	"time"
)

func fixedTime() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) }

func appendEvent(t *testing.T, log *Log, tenant, upload string, kind Kind) Event {
	t.Helper()
	event, err := log.Append(Event{Tenant: tenant, UploadID: upload, Kind: kind, At: fixedTime()})
	if err != nil {
		t.Fatalf("append %s/%s: %v", tenant, upload, err)
	}
	return event
}

func TestReadDoesNotReturnForeignEvents(t *testing.T) {
	log := NewLog()
	appendEvent(t, log, "beta", "u2", UploadOpened) // seq 1, foreign
	appendEvent(t, log, "alpha", "u1", UploadOpened) // seq 2

	items, _ := log.Read(Cursor{Tenant: "alpha", Sequence: 0}, 0)
	if len(items) != 1 || items[0].Tenant != "alpha" {
		t.Fatalf("alpha leaked foreign events: %+v", items)
	}
}

func TestReadDoesNotAdvancePastForeignEvents(t *testing.T) {
	log := NewLog()
	appendEvent(t, log, "alpha", "u1", UploadOpened) // seq 1
	appendEvent(t, log, "beta", "u2", UploadOpened)  // seq 2, foreign
	appendEvent(t, log, "alpha", "u3", PartAccepted) // seq 3
	appendEvent(t, log, "beta", "u4", PartAccepted)  // seq 4, foreign

	// First match: only the alpha event at seq 1; the cursor must stay at 1
	// and not absorb beta's seq 2.
	items, next := log.Read(Cursor{Tenant: "alpha", Sequence: 0}, 1)
	if len(items) != 1 || items[0].UploadID != "u1" {
		t.Fatalf("expected only alpha u1, got %+v", items)
	}
	if next.Sequence != 1 {
		t.Fatalf("alpha cursor advanced past foreign event to %d, want 1", next.Sequence)
	}
	if next.Tenant != "alpha" {
		t.Fatalf("cursor tenant lost: %q", next.Tenant)
	}

	// Resuming must still surface the alpha event at seq 3 despite beta
	// events sitting between it and the cursor.
	items, next = log.Read(next, 1)
	if len(items) != 1 || items[0].UploadID != "u3" {
		t.Fatalf("expected alpha u3 next, got %+v", items)
	}
	if next.Sequence != 3 {
		t.Fatalf("alpha cursor = %d, want 3", next.Sequence)
	}
}

func TestReadGlobalCursorStaysDense(t *testing.T) {
	log := NewLog()
	appendEvent(t, log, "alpha", "u1", UploadOpened) // seq 1
	appendEvent(t, log, "beta", "u2", UploadOpened)  // seq 2

	items, next := log.Read(Cursor{Sequence: 0}, 0)
	if len(items) != 2 {
		t.Fatalf("global cursor expected 2 events, got %d", len(items))
	}
	if next.Sequence != 2 {
		t.Fatalf("global cursor = %d, want 2", next.Sequence)
	}
	if next.Tenant != "" {
		t.Fatalf("global cursor tenant changed to %q", next.Tenant)
	}
}

func TestWaitDoesNotReturnForeignEvents(t *testing.T) {
	log := NewLog()
	appendEvent(t, log, "beta", "u2", UploadOpened) // foreign event present

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	items, next, err := log.Wait(ctx, Cursor{Tenant: "alpha", Sequence: 0})
	if err != context.DeadlineExceeded {
		t.Fatalf("expected deadline exceeded, got err=%v items=%+v", err, items)
	}
	if len(items) != 0 {
		t.Fatalf("alpha should not receive foreign events: %+v", items)
	}
	if next.Sequence != 0 {
		t.Fatalf("alpha cursor advanced during wait to %d", next.Sequence)
	}
}

func TestWaitReturnsOnceTenantEventAppended(t *testing.T) {
	log := NewLog()
	go func() {
		time.Sleep(10 * time.Millisecond)
		_, _ = log.Append(Event{Tenant: "alpha", UploadID: "u1", Kind: UploadOpened, At: fixedTime()})
	}()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	items, next, err := log.Wait(ctx, Cursor{Tenant: "alpha", Sequence: 0})
	if err != nil {
		t.Fatalf("wait: %v", err)
	}
	if len(items) != 1 || items[0].Tenant != "alpha" {
		t.Fatalf("expected one alpha event, got %+v", items)
	}
	if next.Sequence != 1 {
		t.Fatalf("cursor = %d, want 1", next.Sequence)
	}
}
