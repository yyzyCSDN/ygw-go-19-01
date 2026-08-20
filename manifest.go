package multipart

import (
	"sort"
	"time"
)

func BuildManifest(session Session, now time.Time) ObjectManifest {
	parts := append([]Part(nil), session.Parts...)
	sort.Slice(parts, func(i, j int) bool { return parts[i].Number < parts[j].Number })
	manifest := ObjectManifest{UploadID: session.ID, ObjectKey: session.ObjectKey, FinalizedAt: now}
	for _, part := range parts {
		manifest.Parts = append(manifest.Parts, ManifestPart{Number: part.Number, Size: int64(len(part.Data)), SHA256: part.SHA256})
		manifest.TotalSize += int64(len(part.Data))
	}
	return manifest
}
