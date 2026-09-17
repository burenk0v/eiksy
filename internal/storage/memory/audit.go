package memory

import (
    "eiksy/internal/domain/ai"
)

const maxPersistentAuditEvents = 500

func (s *Store) AppendCommandAudit(event ai.CommandAuditEvent) error {
    s.mu.Lock()
    defer s.mu.Unlock()
    // Memory store keeps the audit trail in a bounded in-memory slice.
    // The application-level buffer is still responsible for newest-first presentation.
    s.auditEvents = append(s.auditEvents, event)
    if len(s.auditEvents) > maxPersistentAuditEvents {
        s.auditEvents = append([]ai.CommandAuditEvent(nil), s.auditEvents[len(s.auditEvents)-maxPersistentAuditEvents:]...)
    }
    return nil
}

func (s *Store) CommandAuditTrail() []ai.CommandAuditEvent {
    s.mu.RLock()
    defer s.mu.RUnlock()
    return append([]ai.CommandAuditEvent(nil), s.auditEvents...)
}
