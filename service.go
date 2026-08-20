package multipart

import (
	"context"
	"fmt"
	"time"
)

type Service struct {
	store     *Store
	inspector Inspector
	audit     *AuditLog
	now       func() time.Time
}

func NewService(store *Store, inspector Inspector, audit *AuditLog) (*Service, error) {
	if store == nil || inspector == nil || audit == nil {
		return nil, fmt.Errorf("finalizer dependencies are required")
	}
	return &Service{store: store, inspector: inspector, audit: audit, now: time.Now}, nil
}

func (s *Service) Finalize(ctx context.Context, uploadID string) (ObjectManifest, error) {
	session, err := s.store.Get(uploadID)
	if err != nil {
		return ObjectManifest{}, err
	}
	if session.Status == StatusFinalized {
		// A finalized session has already been committed: its parts were
		// dropped and the manifest is immutable. Re-running finalize would
		// rebuild an empty manifest (no parts left to inspect) and overwrite
		// the good one, so retrying a completed upload must be a no-op.
		return ObjectManifest{}, fmt.Errorf("%w: %s", ErrAlreadyFinalized, uploadID)
	}
	for _, part := range session.Parts {
		if err := s.inspector.Inspect(ctx, part); err != nil {
			return ObjectManifest{}, fmt.Errorf("inspect part %d: %w", part.Number, err)
		}
		s.audit.Record(AuditEvent{UploadID: uploadID, Detail: PartDetail(part), At: s.now()})
	}
	manifest := BuildManifest(session, s.now())
	if err := s.store.SaveFinal(uploadID, manifest); err != nil {
		return ObjectManifest{}, err
	}
	return cloneManifest(manifest), nil
}
