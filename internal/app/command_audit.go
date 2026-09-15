package app

import (
	"regexp"
	"strings"
	"sync"
	"time"

	"eiksy/internal/domain/ai"
)

const maxCommandAuditEvents = 500

type commandAuditBuffer struct {
	mu     sync.RWMutex
	events []ai.CommandAuditEvent
}

var commandAuditBuffers sync.Map // map[*Service]*commandAuditBuffer

var sensitiveCommandPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(password|passwd|token|secret|api[_-]?key|authorization)\s*=\s*[^\s]+`),
	regexp.MustCompile(`(?i)(password|passwd|token|secret|api[_-]?key)\s+[^\s]+`),
	regexp.MustCompile(`(?i)(authorization\s*:\s*(?:bearer|basic)\s+)[^\s'\"]+`),
}

func (s *Service) recordCommandAudit(providerID, sessionID, command, policyDecision, approval, result string, exitCode int, durationMs int64, extra ai.CommandAuditEvent) {
	if s == nil {
		return
	}
	value, _ := commandAuditBuffers.LoadOrStore(s, &commandAuditBuffer{})
	buffer := value.(*commandAuditBuffer)
	chatSessionID := ""
	if s.store != nil {
		chatSessionID = strings.TrimSpace(s.store.AIState().ChatSessionID)
	}
	event := ai.CommandAuditEvent{
		ID:             "audit-cmd-" + time.Now().UTC().Format("20060102T150405.000000000Z"),
		At:             time.Now().UTC().Format(time.RFC3339Nano),
		ChatSessionID:  chatSessionID,
		ProviderID:     strings.TrimSpace(providerID),
		ToolID:         nativeSSHExecPolicyToolID,
		SessionID:      strings.TrimSpace(sessionID),
		Command:        redactCommand(command),
		PolicyDecision: strings.TrimSpace(policyDecision),
		Approval:       strings.TrimSpace(approval),
		Result:         strings.TrimSpace(result),
		ExitCode:       exitCode,
		DurationMs:     durationMs,
		ErrorType:      strings.TrimSpace(extra.ErrorType),
		Error:          strings.TrimSpace(extra.Error),
	}
	buffer.mu.Lock()
	defer buffer.mu.Unlock()
	buffer.events = append(buffer.events, event)
	if len(buffer.events) > maxCommandAuditEvents {
		buffer.events = append([]ai.CommandAuditEvent(nil), buffer.events[len(buffer.events)-maxCommandAuditEvents:]...)
	}
}

// GetCommandAuditTrail returns the bounded in-memory audit trail in newest-first order.
func (s *Service) GetCommandAuditTrail() []ai.CommandAuditEvent {
	if s == nil {
		return nil
	}
	value, ok := commandAuditBuffers.Load(s)
	if !ok {
		return []ai.CommandAuditEvent{}
	}
	buffer := value.(*commandAuditBuffer)
	buffer.mu.RLock()
	defer buffer.mu.RUnlock()
	result := make([]ai.CommandAuditEvent, 0, len(buffer.events))
	for i := len(buffer.events) - 1; i >= 0; i-- {
		result = append(result, buffer.events[i])
	}
	return result
}

func redactCommand(command string) string {
	redacted := strings.TrimSpace(command)
	for _, pattern := range sensitiveCommandPatterns {
		redacted = pattern.ReplaceAllStringFunc(redacted, func(match string) string {
			if index := strings.IndexAny(match, "=:"); index >= 0 {
				return match[:index+1] + "[REDACTED]"
			}
			parts := strings.Fields(match)
			if len(parts) > 1 {
				return parts[0] + " [REDACTED]"
			}
			return "[REDACTED]"
		})
	}
	return redacted
}
