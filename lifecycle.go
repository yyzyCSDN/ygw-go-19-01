package multipart

import (
	"fmt"
	"time"
)

type LifecycleState string

const (
	LifecycleOpen       LifecycleState = "open"
	LifecycleInspecting LifecycleState = "inspecting"
	LifecycleCommitting LifecycleState = "committing"
	LifecycleComplete   LifecycleState = "complete"
	LifecycleFailed     LifecycleState = "failed"
)

type Lifecycle struct {
	UploadID  string
	State     LifecycleState
	Revision  uint64
	UpdatedAt time.Time
	LastError string
}

func NewLifecycle(uploadID string, now time.Time) Lifecycle {
	return Lifecycle{UploadID: uploadID, State: LifecycleOpen, Revision: 1, UpdatedAt: now}
}

func (l Lifecycle) Transition(next LifecycleState, now time.Time, cause error) (Lifecycle, error) {
	if !validLifecycleTransition(l.State, next) {
		return Lifecycle{}, fmt.Errorf("invalid lifecycle transition %s -> %s", l.State, next)
	}
	l.State = next
	l.Revision++
	l.UpdatedAt = now
	l.LastError = ""
	if cause != nil {
		l.LastError = cause.Error()
	}
	return l, nil
}

func validLifecycleTransition(current, next LifecycleState) bool {
	switch current {
	case LifecycleOpen:
		return next == LifecycleInspecting || next == LifecycleFailed
	case LifecycleInspecting:
		return next == LifecycleCommitting || next == LifecycleFailed
	case LifecycleCommitting:
		return next == LifecycleComplete || next == LifecycleFailed
	case LifecycleComplete, LifecycleFailed:
		// Terminal states are closed to forward progress: an upload that
		// already reached complete or failed must never be walked back into
		// inspecting or committing, nor reset to open. Otherwise the retry
		// loop resurrects finished uploads, regenerates their manifests, and
		// accumulates dirty data.
		//
		// The only permitted moves are idempotent retries:
		//   - complete -> complete: re-dispatching finalize after a partially
		//     durable commit lands back on the same terminal state.
		//   - failed -> complete: a retried upload that succeeds this time.
		//   - failed -> failed: recording a repeated failure on a session that
		//     is already failed (attempts are tracked in the recovery book).
		if next == LifecycleComplete {
			return true
		}
		return current == LifecycleFailed && next == LifecycleFailed
	default:
		return false
	}
}

func (l Lifecycle) Terminal() bool {
	return l.State == LifecycleComplete || l.State == LifecycleFailed
}

func (l Lifecycle) CanRetry(maxAttempts, attempts int) bool {
	return l.State == LifecycleFailed && attempts < maxAttempts
}
