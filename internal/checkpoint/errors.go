package checkpoint

import "errors"

var (
	// ErrRevisionRegression is returned when Save is called with a record
	// whose Revision is smaller than the one already persisted for the same
	// upload. The older write is rejected so a replayed or out-of-order
	// attempt can never clobber a newer checkpoint.
	ErrRevisionRegression = errors.New("checkpoint revision regression")
	// ErrRevisionConflict is returned when Save is called with a record
	// whose Revision matches the one already persisted. Equal revisions are
	// rejected explicitly rather than silently replacing the stored value,
	// so callers can distinguish a replay from a genuine advance.
	ErrRevisionConflict = errors.New("checkpoint revision conflict")
)
