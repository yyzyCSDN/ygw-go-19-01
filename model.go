package multipart

import "time"

type UploadStatus string

const (
	StatusUploading UploadStatus = "uploading"
	StatusFinalized UploadStatus = "finalized"
)

type PartMetadata struct{ Region, StorageClass string }

type Part struct {
	Number   int
	Data     []byte
	SHA256   string
	Metadata *PartMetadata
}

type ManifestPart struct {
	Number int    `json:"number"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}
type ObjectManifest struct {
	UploadID    string         `json:"upload_id"`
	ObjectKey   string         `json:"object_key"`
	TotalSize   int64          `json:"total_size"`
	Parts       []ManifestPart `json:"parts"`
	FinalizedAt time.Time      `json:"finalized_at"`
}
type Session struct {
	ID, ObjectKey string
	ExpectedParts int
	Status        UploadStatus
	Parts         []Part
	Manifest      *ObjectManifest
}

func clonePart(value Part) Part {
	out := value
	out.Data = append([]byte(nil), value.Data...)
	if value.Metadata != nil {
		metadata := *value.Metadata
		out.Metadata = &metadata
	}
	return out
}
func cloneManifest(value ObjectManifest) ObjectManifest {
	out := value
	out.Parts = append([]ManifestPart(nil), value.Parts...)
	return out
}
func cloneSession(value Session) Session {
	out := value
	out.Parts = make([]Part, len(value.Parts))
	for index, part := range value.Parts {
		out.Parts[index] = clonePart(part)
	}
	if value.Manifest != nil {
		manifest := cloneManifest(*value.Manifest)
		out.Manifest = &manifest
	}
	return out
}
