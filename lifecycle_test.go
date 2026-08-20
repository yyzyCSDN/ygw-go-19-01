package multipart

import (
	"testing"
	"time"
)

func TestLifecycleTransitionsForwardProgress(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	cases := []struct {
		name    string
		current LifecycleState
		next    LifecycleState
		want    bool
	}{
		{"open to inspecting", LifecycleOpen, LifecycleInspecting, true},
		{"open to failed", LifecycleOpen, LifecycleFailed, true},
		{"inspecting to committing", LifecycleInspecting, LifecycleCommitting, true},
		{"inspecting to failed", LifecycleInspecting, LifecycleFailed, true},
		{"committing to complete", LifecycleCommitting, LifecycleComplete, true},
		{"committing to failed", LifecycleCommitting, LifecycleFailed, true},

		// Idempotent terminal retries must remain allowed.
		{"complete to complete (retry)", LifecycleComplete, LifecycleComplete, true},
		{"failed to complete (retry success)", LifecycleFailed, LifecycleComplete, true},
		{"failed to failed (repeated failure)", LifecycleFailed, LifecycleFailed, true},

		// A finished upload must never be walked back into the processing
		// states or reset to open — that is what lets retry resurrect it.
		{"complete to inspecting", LifecycleComplete, LifecycleInspecting, false},
		{"complete to committing", LifecycleComplete, LifecycleCommitting, false},
		{"complete to open", LifecycleComplete, LifecycleOpen, false},
		{"complete to failed", LifecycleComplete, LifecycleFailed, false},
		{"failed to inspecting", LifecycleFailed, LifecycleInspecting, false},
		{"failed to committing", LifecycleFailed, LifecycleCommitting, false},
		{"failed to open", LifecycleFailed, LifecycleOpen, false},

		{"open to complete", LifecycleOpen, LifecycleComplete, false},
		{"inspecting to complete", LifecycleInspecting, LifecycleComplete, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := validLifecycleTransition(tc.current, tc.next)
			if got != tc.want {
				t.Fatalf("validLifecycleTransition(%s, %s) = %v, want %v", tc.current, tc.next, got, tc.want)
			}
			lc := Lifecycle{UploadID: "u", State: tc.current, Revision: 1, UpdatedAt: now}
			_, err := lc.Transition(tc.next, now, nil)
			if tc.want && err != nil {
				t.Fatalf("Transition(%s, %s) unexpected error: %v", tc.current, tc.next, err)
			}
			if !tc.want && err == nil {
				t.Fatalf("Transition(%s, %s) expected error, got nil", tc.current, tc.next)
			}
		})
	}
}

func TestLifecycleTerminalFlags(t *testing.T) {
	if !(Lifecycle{State: LifecycleComplete}.Terminal() && Lifecycle{State: LifecycleFailed}.Terminal()) {
		t.Fatal("complete and failed must be terminal")
	}
	if (Lifecycle{State: LifecycleOpen}.Terminal() ||
		Lifecycle{State: LifecycleInspecting}.Terminal() ||
		Lifecycle{State: LifecycleCommitting}.Terminal()) {
		t.Fatal("non-terminal states reported terminal")
	}
	if !(Lifecycle{State: LifecycleFailed}).CanRetry(3, 1) {
		t.Fatal("failed session within attempt budget should retry")
	}
	if (Lifecycle{State: LifecycleFailed}).CanRetry(3, 3) {
		t.Fatal("failed session past attempt budget should not retry")
	}
	if (Lifecycle{State: LifecycleComplete}).CanRetry(3, 0) {
		t.Fatal("completed session should not be eligible for retry")
	}
}
