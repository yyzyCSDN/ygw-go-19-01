package observability

import (
	"sort"
	"time"
)

type ComponentState string

const (
	StateReady    ComponentState = "ready"
	StateDegraded ComponentState = "degraded"
	StateStopped  ComponentState = "stopped"
)

type Component struct {
	Name      string         `json:"name"`
	State     ComponentState `json:"state"`
	Detail    string         `json:"detail,omitempty"`
	CheckedAt time.Time      `json:"checked_at"`
}

type Health struct {
	Status     ComponentState `json:"status"`
	Components []Component    `json:"components"`
	CheckedAt  time.Time      `json:"checked_at"`
}

func SummarizeHealth(components []Component, now time.Time) Health {
	result := Health{Status: StateReady, CheckedAt: now, Components: append([]Component(nil), components...)}
	sort.Slice(result.Components, func(i, j int) bool { return result.Components[i].Name < result.Components[j].Name })
	for _, component := range result.Components {
		if component.State == StateStopped {
			result.Status = StateStopped
			break
		}
		if component.State == StateDegraded {
			result.Status = StateDegraded
		}
	}
	return result
}

func (h Health) Ready() bool { return h.Status == StateReady }
