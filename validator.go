package multipart

import (
	"encoding/hex"
	"fmt"
	"strings"
)

func ValidateSession(session Session) error {
	if session.ID == "" || session.ExpectedParts <= 0 || len(session.Parts) != session.ExpectedParts {
		return fmt.Errorf("%w: expected %d parts, got %d", ErrInvalidSession, session.ExpectedParts, len(session.Parts))
	}
	seen := map[int]struct{}{}
	for _, part := range session.Parts {
		if part.Number < 1 || part.Number > session.ExpectedParts || len(part.Data) == 0 {
			return fmt.Errorf("%w: part %d", ErrInvalidPart, part.Number)
		}
		if _, exists := seen[part.Number]; exists {
			return fmt.Errorf("%w: duplicate part %d", ErrInvalidPart, part.Number)
		}
		seen[part.Number] = struct{}{}
		digest, err := hex.DecodeString(part.SHA256)
		if err != nil || len(digest) != 32 {
			return fmt.Errorf("%w: checksum format for part %d", ErrInvalidPart, part.Number)
		}
		if !strings.EqualFold(Digest(part.Data), part.SHA256) {
			return fmt.Errorf("%w: part %d", ErrChecksum, part.Number)
		}
	}
	return nil
}
