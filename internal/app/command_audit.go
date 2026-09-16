package app

import (
	"regexp"
	"strings"
	"sync"
	"time"

	"eiksy/internal/domain/ai"
)

const maxCommandAuditEvents = 500
const maxCommandAuditText = 4096

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
		ChatSessionID:  redactAuditValue(chatSessionID),
		ProviderID:     redactAuditValue(providerID),
		ToolID:         nativeSSHExecPolicyToolID,
		SessionID:      redactAuditValue(sessionID),
		Command:        redactAuditValue(command),
		PolicyDecision: redactAuditValue(policyDecision),
		Approval:       redactAuditValue(approval),
		Result:         redactAuditOutput(result),
		ExitCode:       exitCode,
		DurationMs:     durationMs,
		ErrorType:      redactAuditValue(extra.ErrorType),
		Error:          redactAuditOutput(extra.Error),
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

// redactAuditValue is the final audit-boundary sanitizer for structured text
// that can be safely retained after removing known sensitive values.
func redactAuditValue(value string) string {
	redacted := redactCommand(strings.TrimSpace(value))
	redacted = strings.Map(func(r rune) rune {
		switch r {
		case '\n', '\r', '\t':
			return ' '
		default:
			if r < 0x20 || r == 0x7f {
				return -1
			}
			return r
		}
	}, redacted)
	if len(redacted) > maxCommandAuditText {
		redacted = redacted[:maxCommandAuditText] + "...[TRUNCATED]"
	}
	return redacted
}

// redactAuditOutput treats command output and execution errors as sensitive by
// default. They are arbitrary external text and may contain credentials that
// cannot be reliably identified by pattern matching.
func redactAuditOutput(value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	return "[REDACTED]"
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
