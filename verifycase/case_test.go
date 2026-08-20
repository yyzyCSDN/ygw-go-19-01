package verifycase

import (
	"context"
	"testing"
	"time"

	checkpoint "example.com/partflow/internal/checkpoint"
	"example.com/partflow/internal/scheduler"
)

func TestStaleStateCannotOverwriteNew(t *testing.T) {
	ctx := context.Background()
	now := time.Now().Truncate(time.Millisecond)
	store := checkpoint.NewMemoryStore()
	newer := checkpoint.Record{UploadID: "u1", Stage: checkpoint.StageInspect, Part: 2, Revision: 2, SavedAt: now}
	if err := store.Save(ctx, newer); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(ctx, checkpoint.Record{UploadID: "u1", Stage: checkpoint.StageAccepted, Part: 1, Revision: 2, SavedAt: now}); err == nil {
		t.Fatal("equal-revision checkpoint overwrite accepted")
	}
	if err := store.Save(ctx, checkpoint.Record{UploadID: "u1", Stage: checkpoint.StageAccepted, Part: 1, Revision: 1, SavedAt: now}); err == nil {
		t.Fatal("lower-revision checkpoint overwrite accepted")
	}
	records, err := store.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 || records[0].Revision != 2 || records[0].Stage != checkpoint.StageInspect {
		t.Fatalf("persisted checkpoint = %+v", records)
	}

	queue := scheduler.NewQueue()
	created := now
	if err := queue.Enqueue(scheduler.Job{ID: "j1", Tenant: "t", UploadID: "u1", Priority: 1, Attempt: 2, CreatedAt: created}); err != nil {
		t.Fatal(err)
	}
	if err := queue.Enqueue(scheduler.Job{ID: "j1", Tenant: "t", UploadID: "u1", Priority: 1, Attempt: 1, CreatedAt: created}); err == nil {
		t.Fatal("scheduler attempt regression accepted")
	}
	if queue.Len() != 1 {
		t.Fatalf("queue len=%d, want 1", queue.Len())
	}
}
