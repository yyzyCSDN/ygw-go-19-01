package multipart

import "errors"

var (
	ErrInvalidSession   = errors.New("invalid multipart session")
	ErrSessionNotFound  = errors.New("multipart session not found")
	ErrInvalidPart      = errors.New("invalid multipart part")
	ErrChecksum         = errors.New("multipart checksum mismatch")
	ErrAlreadyFinalized = errors.New("multipart session already finalized")
)
