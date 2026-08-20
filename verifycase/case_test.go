package verifycase

import (
	"context"
	"errors"
	"testing"
	"time"

	"example.com/partflow"
)

func TestLifecycleTerminalTransitionProtection(t *testing.T) {
	store := multipart.NewStore()
	service, err := multipart.NewService(store, multipart.BasicInspector{MaxPartBytes: 1024}, multipart.NewAuditLog())
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Begin("u1", "o1", 1); err != nil {
		t.Fatal(err)
	}
	data := []byte("payload")
	if err := store.AddPart("u1", multipart.Part{Number: 1, Data: data, SHA256: multipart.Digest(data)}); err != nil {
		t.Fatal(err)
	}
	manifest, err := service.Finalize(context.Background(), "u1")
	if err != nil || manifest.TotalSize != int64(len(data)) {
		t.Fatalf("finalize err=%v manifest=%+v", err, manifest)
	}
	if _, err := service.Finalize(context.Background(), "u1"); !errors.Is(err, multipart.ErrAlreadyFinalized) {
		t.Fatalf("second finalize err=%v, want ErrAlreadyFinalized", err)
	}

	now := time.Now().Truncate(time.Millisecond)
	lifecycle := multipart.NewLifecycle("u1", now)
	var transitionErr error
	lifecycle, transitionErr = lifecycle.Transition(multipart.LifecycleInspecting, now.Add(time.Second), nil)
	if transitionErr != nil {
		t.Fatal(transitionErr)
	}
	lifecycle, transitionErr = lifecycle.Transition(multipart.LifecycleCommitting, now.Add(2*time.Second), nil)
	if transitionErr != nil {
		t.Fatal(transitionErr)
	}
	lifecycle, transitionErr = lifecycle.Transition(multipart.LifecycleComplete, now.Add(3*time.Second), nil)
	if transitionErr != nil {
		t.Fatal(transitionErr)
	}
	if !lifecycle.Terminal() {
		t.Fatal("complete lifecycle is not terminal")
	}
	if _, transitionErr := lifecycle.Transition(multipart.LifecycleInspecting, now.Add(4*time.Second), nil); transitionErr == nil {
		t.Fatal("transition out of complete succeeded")
	}
}
