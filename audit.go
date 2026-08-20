package multipart

import (
	"fmt"
	"sync"
	"time"
)

type AuditEvent struct {
	UploadID, Detail string
	At               time.Time
}
type AuditLog struct {
	mu     sync.RWMutex
	events []AuditEvent
}

func NewAuditLog() *AuditLog { return &AuditLog{} }
func (l *AuditLog) Record(event AuditEvent) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.events = append(l.events, event)
}
func (l *AuditLog) Events() []AuditEvent {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return append([]AuditEvent(nil), l.events...)
}
func PartDetail(part Part) string {
	if part.Metadata == nil {
		return fmt.Sprintf("part=%d region=unknown", part.Number)
	}
	return fmt.Sprintf("part=%d region=%s", part.Number, part.Metadata.Region)
}
