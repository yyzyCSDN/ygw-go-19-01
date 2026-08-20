package multipart

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"
)

type Snapshot struct {
	Version    int              `json:"version"`
	CapturedAt time.Time        `json:"captured_at"`
	Sessions   []Session        `json:"sessions"`
	Lifecycles []Lifecycle      `json:"lifecycles"`
	Recoveries []RecoveryRecord `json:"recoveries"`
}

func BuildSnapshot(sessions []Session, lifecycles []Lifecycle, recoveries []RecoveryRecord, now time.Time) Snapshot {
	snapshot := Snapshot{Version: 1, CapturedAt: now}
	snapshot.Sessions = make([]Session, len(sessions))
	for index, session := range sessions {
		snapshot.Sessions[index] = cloneSession(session)
	}
	snapshot.Lifecycles = append([]Lifecycle(nil), lifecycles...)
	snapshot.Recoveries = append([]RecoveryRecord(nil), recoveries...)
	sort.Slice(snapshot.Sessions, func(i, j int) bool { return snapshot.Sessions[i].ID < snapshot.Sessions[j].ID })
	sort.Slice(snapshot.Lifecycles, func(i, j int) bool { return snapshot.Lifecycles[i].UploadID < snapshot.Lifecycles[j].UploadID })
	sort.Slice(snapshot.Recoveries, func(i, j int) bool { return snapshot.Recoveries[i].UploadID < snapshot.Recoveries[j].UploadID })
	return snapshot
}

func EncodeSnapshot(snapshot Snapshot) ([]byte, error) {
	if err := ValidateSnapshot(snapshot); err != nil {
		return nil, err
	}
	data, err := json.Marshal(snapshot)
	if err != nil {
		return nil, fmt.Errorf("encode snapshot: %w", err)
	}
	return data, nil
}

func DecodeSnapshot(data []byte) (Snapshot, error) {
	var snapshot Snapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return Snapshot{}, fmt.Errorf("decode snapshot: %w", err)
	}
	if err := ValidateSnapshot(snapshot); err != nil {
		return Snapshot{}, err
	}
	return BuildSnapshot(snapshot.Sessions, snapshot.Lifecycles, snapshot.Recoveries, snapshot.CapturedAt), nil
}

func ValidateSnapshot(snapshot Snapshot) error {
	if snapshot.Version != 1 || snapshot.CapturedAt.IsZero() {
		return fmt.Errorf("unsupported snapshot")
	}
	seen := make(map[string]struct{}, len(snapshot.Sessions))
	for _, session := range snapshot.Sessions {
		if session.ID == "" {
			return fmt.Errorf("snapshot contains empty session id")
		}
		if _, ok := seen[session.ID]; ok {
			return fmt.Errorf("duplicate session %s", session.ID)
		}
		seen[session.ID] = struct{}{}
	}
	for _, lifecycle := range snapshot.Lifecycles {
		if _, ok := seen[lifecycle.UploadID]; !ok {
			return fmt.Errorf("orphan lifecycle %s", lifecycle.UploadID)
		}
	}
	return nil
}
