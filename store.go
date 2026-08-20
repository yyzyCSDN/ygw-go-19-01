package multipart

import (
	"fmt"
	"sync"
)

type Store struct {
	mu       sync.RWMutex
	sessions map[string]Session
}

func NewStore() *Store { return &Store{sessions: map[string]Session{}} }

func (s *Store) Begin(id, objectKey string, expectedParts int) error {
	if id == "" || objectKey == "" || expectedParts <= 0 {
		return ErrInvalidSession
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[id] = Session{ID: id, ObjectKey: objectKey, ExpectedParts: expectedParts, Status: StatusUploading}
	return nil
}

func (s *Store) AddPart(uploadID string, part Part) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	session, ok := s.sessions[uploadID]
	if !ok {
		return fmt.Errorf("%w: %s", ErrSessionNotFound, uploadID)
	}
	session.Parts = append(session.Parts, clonePart(part))
	s.sessions[uploadID] = cloneSession(session)
	return nil
}

func (s *Store) Get(id string) (Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	session, ok := s.sessions[id]
	if !ok {
		return Session{}, fmt.Errorf("%w: %s", ErrSessionNotFound, id)
	}
	return cloneSession(session), nil
}

func (s *Store) SaveFinal(id string, manifest ObjectManifest) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	session, ok := s.sessions[id]
	if !ok {
		return ErrSessionNotFound
	}
	if session.Status == StatusFinalized {
		return fmt.Errorf("%w: %s", ErrAlreadyFinalized, id)
	}
	session.Status = StatusFinalized
	session.Manifest = &manifest
	session.Parts = nil
	session.ExpectedParts = 0
	s.sessions[id] = cloneSession(session)
	return nil
}
