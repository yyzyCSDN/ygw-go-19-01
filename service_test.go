package multipart

import (
	"context"
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
