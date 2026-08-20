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
