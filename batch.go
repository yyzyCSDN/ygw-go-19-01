package multipart

import (
	"context"
	"fmt"
)

func (s *Service) FinalizeBatch(ctx context.Context, uploadIDs []string) ([]ObjectManifest, error) {
	seen := map[string]struct{}{}
	for _, id := range uploadIDs {
		if _, exists := seen[id]; exists {
			return nil, fmt.Errorf("%w: duplicate upload %s", ErrInvalidSession, id)
		}
		seen[id] = struct{}{}
	}
	results := make([]ObjectManifest, 0, len(uploadIDs))
	for _, id := range uploadIDs {
		manifest, err := s.Finalize(ctx, id)
		if err != nil {
			return results, err
		}
		results = append(results, manifest)
	}
	return results, nil
}
