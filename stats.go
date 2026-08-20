package multipart

type Stats struct {
	Objects, Parts int
	Bytes          int64
}

func Summarize(manifests []ObjectManifest) Stats {
	var stats Stats
	for _, manifest := range manifests {
		stats.Objects++
		stats.Parts += len(manifest.Parts)
		stats.Bytes += manifest.TotalSize
	}
	return stats
}
