package scheduler

import "errors"

var (
	// ErrAttemptRegression is returned when Enqueue is called with a job
	// whose Attempt is smaller than the one already queued for the same ID.
	// The older attempt is rejected so a replayed or out-of-order retry can
	// never revive a superseded attempt.
	ErrAttemptRegression = errors.New("scheduler attempt regression")
	// ErrAttemptConflict is returned when Enqueue is called with a job whose
	// Attempt matches the one already queued. Equal attempts are rejected
	// explicitly rather than silently replacing the queued job, so callers
	// can tell a duplicate from a genuine retry.
	ErrAttemptConflict = errors.New("scheduler attempt conflict")
)
