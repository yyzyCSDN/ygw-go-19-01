package main

import (
	"context"
	"fmt"

	multipart "example.com/partflow"
)

func main() {
	store := multipart.NewStore()
	_ = store.Begin("demo-upload", "videos/demo.mp4", 2)
	first := []byte("video-")
	second := []byte("payload")
	_ = store.AddPart("demo-upload", multipart.Part{Number: 1, Data: first, SHA256: multipart.Digest(first)})
	_ = store.AddPart("demo-upload", multipart.Part{Number: 2, Data: second, SHA256: multipart.Digest(second)})
	service, _ := multipart.NewService(store, multipart.BasicInspector{MaxPartBytes: 1024}, multipart.NewAuditLog())
	manifest, err := service.Finalize(context.Background(), "demo-upload")
	if err != nil {
		panic(err)
	}
	fmt.Printf("finalized %s: parts=%d bytes=%d\n", manifest.ObjectKey, len(manifest.Parts), manifest.TotalSize)
}
