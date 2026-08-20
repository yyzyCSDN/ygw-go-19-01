package multipart

import (
	"encoding/json"
	"fmt"
)

func EncodeManifest(manifest ObjectManifest) ([]byte, error) {
	data, err := json.Marshal(cloneManifest(manifest))
	if err != nil {
		return nil, fmt.Errorf("encode manifest: %w", err)
	}
	return data, nil
}
func DecodeManifest(data []byte) (ObjectManifest, error) {
	var manifest ObjectManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return ObjectManifest{}, fmt.Errorf("decode manifest: %w", err)
	}
	return cloneManifest(manifest), nil
}
