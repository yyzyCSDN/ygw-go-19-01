package multipart

import (
	"context"
	"errors"
	"testing"
)

func testService(t *testing.T, parts ...Part) (*Service, *Store, *AuditLog) {
	t.Helper()
	store := NewStore()
	if err := store.Begin("upload-1", "archives/db.tar", len(parts)); err != nil {
		t.Fatal(err)
	}
	for _, part := range parts {
		if part.SHA256 == "" {
			part.SHA256 = Digest(part.Data)
		}
		if err := store.AddPart("upload-1", part); err != nil {
			t.Fatal(err)
		}
	}
	audit := NewAuditLog()
	service, err := NewService(store, BasicInspector{MaxPartBytes: 1024}, audit)
	if err != nil {
		t.Fatal(err)
	}
	return service, store, audit
}

func TestFinalizesValidPartsInNumberOrder(t *testing.T) {
	service, store, audit := testService(t, Part{Number: 2, Data: []byte("world"), Metadata: &PartMetadata{Region: "west"}}, Part{Number: 1, Data: []byte("hello"), Metadata: &PartMetadata{Region: "east"}})
	manifest, err := service.Finalize(context.Background(), "upload-1")
	if err != nil {
		t.Fatal(err)
	}
	if manifest.TotalSize != 10 || len(manifest.Parts) != 2 || len(audit.Events()) != 2 {
		t.Fatalf("manifest=%+v", manifest)
	}
	session, _ := store.Get("upload-1")
	if session.Status != StatusFinalized {
		t.Fatalf("session=%+v", session)
	}
}

func TestManifestCodec(t *testing.T) {
	manifest := ObjectManifest{UploadID: "u", ObjectKey: "a", Parts: []ManifestPart{{Number: 1, Size: 2, SHA256: Digest([]byte("ok"))}}}
	data, err := EncodeManifest(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeManifest(data); err != nil {
		t.Fatal(err)
	}
}

func TestFinalizeRejectsAlreadyFinalizedSession(t *testing.T) {
	service, store, audit := testService(t, Part{Number: 1, Data: []byte("hello"), Metadata: &PartMetadata{Region: "east"}})

	first, err := service.Finalize(context.Background(), "upload-1")
	if err != nil {
		t.Fatalf("first finalize: %v", err)
	}
	if first.TotalSize != 5 || len(first.Parts) != 1 || len(audit.Events()) != 1 {
		t.Fatalf("first manifest=%+v events=%d", first, len(audit.Events()))
	}

	// A retry that picks up the already-completed upload must not re-run
	// inspect/audit/manifest and must surface a clear error instead of
	// overwriting the committed manifest with a regenerated (empty) one.
	eventsBefore := len(audit.Events())
	_, err = service.Finalize(context.Background(), "upload-1")
	if !errors.Is(err, ErrAlreadyFinalized) {
		t.Fatalf("second finalize: expected ErrAlreadyFinalized, got %v", err)
	}
	if len(audit.Events()) != eventsBefore {
		t.Fatalf("second finalize recorded %d new audit events, want 0", len(audit.Events())-eventsBefore)
	}
	session, _ := store.Get("upload-1")
	if session.Status != StatusFinalized || session.Manifest == nil || session.Manifest.TotalSize != 5 {
		t.Fatalf("committed manifest was mutated by rejected retry: %+v", session)
	}
}

func TestSaveFinalRejectsAlreadyFinalized(t *testing.T) {
	store := NewStore()
	if err := store.Begin("upload-1", "archives/db.tar", 1); err != nil {
		t.Fatal(err)
	}
	manifest := ObjectManifest{UploadID: "upload-1", ObjectKey: "archives/db.tar", Parts: []ManifestPart{{Number: 1, Size: 3}}}
	if err := store.SaveFinal("upload-1", manifest); err != nil {
		t.Fatalf("first SaveFinal: %v", err)
	}
	if err := store.SaveFinal("upload-1", manifest); !errors.Is(err, ErrAlreadyFinalized) {
		t.Fatalf("second SaveFinal: expected ErrAlreadyFinalized, got %v", err)
	}
}
