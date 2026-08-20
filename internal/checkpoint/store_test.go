package checkpoint

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func validRecord(uploadID string, revision uint64) Record {
	return Record{
		UploadID: uploadID,
		Stage:    StageInspect,
		Part:      1,
		Revision:  revision,
		SavedAt:  time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC),
	}
}

func TestSaveAcceptsStrictlyGreaterRevision(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	if err := store.Save(ctx, validRecord("u", 1)); err != nil {
		t.Fatalf("save revision 1: %v", err)
	}
	if err := store.Save(ctx, validRecord("u", 3)); err != nil {
		t.Fatalf("save revision 3: %v", err)
	}
	record, ok, err := store.Load(ctx, "u")
	if err != nil || !ok {
		t.Fatalf("load: ok=%v err=%v", ok, err)
	}
	if record.Revision != 3 {
		t.Fatalf("revision=%d want 3", record.Revision)
	}
}

func TestSaveRejectsEqualRevision(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	if err := store.Save(ctx, validRecord("u", 2)); err != nil {
		t.Fatalf("save revision 2: %v", err)
	}
	// Same revision with a newer stage must not silently replace the stored
	// value; equal revisions are rejected explicitly.
	err := store.Save(ctx, Record{
		UploadID: "u",
		Stage:    StagePersisted,
		Part:      1,
		Revision:  2,
		SavedAt:   time.Date(2026, 8, 19, 13, 0, 0, 0, time.UTC),
	})
	if !errors.Is(err, ErrRevisionConflict) {
		t.Fatalf("err=%v want ErrRevisionConflict", err)
	}
	record, _, _ := store.Load(ctx, "u")
	if record.Stage != StageInspect {
		t.Fatalf("stage=%q want %q (silent replace happened)", record.Stage, StageInspect)
	}
}

func TestSaveRejectsRegression(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	if err := store.Save(ctx, validRecord("u", 5)); err != nil {
		t.Fatalf("save revision 5: %v", err)
	}
	err := store.Save(ctx, validRecord("u", 4))
	if !errors.Is(err, ErrRevisionRegression) {
		t.Fatalf("err=%v want ErrRevisionRegression", err)
	}
	record, _, _ := store.Load(ctx, "u")
	if record.Revision != 5 {
		t.Fatalf("revision=%d want 5 (regression clobbered newer stage)", record.Revision)
	}
}

// TestSaveMonotonicUnderConcurrency asserts that no matter how concurrently
// reordered the saves arrive, the persisted revision is always the maximum:
// the largest revision always wins once stored, and nothing smaller can ever
// replace it.
func TestSaveMonotonicUnderConcurrency(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	const workers = 32
	const revisions = 64

	var wg sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		wg.Add(1)
		go func(base int) {
			defer wg.Done()
			for r := 1; r <= revisions; r++ {
				// Distinct revision per save so the final state is
				// deterministic regardless of scheduling.
				rev := uint64(base*revisions + r)
				_ = store.Save(ctx, validRecord("shared", rev))
			}
		}(worker)
	}
	wg.Wait()

	record, ok, err := store.Load(ctx, "shared")
	if err != nil || !ok {
		t.Fatalf("load: ok=%v err=%v", ok, err)
	}
	want := uint64(workers * revisions)
	if record.Revision != want {
		t.Fatalf("revision=%d want %d (monotonic gate leaked)", record.Revision, want)
	}
}
