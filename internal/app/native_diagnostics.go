package app

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"eiksy/internal/domain/ai"
	"eiksy/internal/domain/sessions"
)

const maxAIDiagnosticOutput = 4096

type aiDiagnosticCheck struct {
	Name   string                           `json:"name"`
	Command string                           `json:"command"`
	Result sessions.CommandExecutionResult `json:"result"`
}

// dispatchNativeDiagnostics executes a fixed read-only diagnostic set. The AI
// never supplies the commands themselves; every fixed command is still passed
// through the existing Command Policy before execution.
func (s *Service) emitNativeDiagnosticsOperation(status, sessionID string, checks int, approval, message string, durationMs int64) {
	command := fmt.Sprintf("%s summary (%d checks)", nativeSSHDiagnosticsToolName, checks)
	s.emitNativeOperation(status, sessionID, command, sessions.CommandExecutionResult{ExitCode: -1, DurationMs: durationMs}, approval, message)
}

func (s *Service) dispatchNativeDiagnostics(call nativeToolCall, policy ai.CommandPolicy, activeSessionID, providerID string) (string, bool, error) {
	var args struct {
		SessionID string `json:"sessionId"`
		Operation string `json:"operation"`
	}
	decoder := json.NewDecoder(strings.NewReader(call.Function.Arguments))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&args); err != nil {
		return marshalNativeToolError("invalid arguments: %v", err), false, nil
	}
	args.SessionID = strings.TrimSpace(args.SessionID)
	args.Operation = strings.TrimSpace(args.Operation)
	if args.SessionID == "" {
		args.SessionID = strings.TrimSpace(activeSessionID)
	}
	if args.SessionID == "" || args.Operation == "" {
		return `{"error":{"type":"invalid_request","message":"sessionId and operation are required"}}`, false, nil
	}
	if args.Operation != "summary" {
		return `{"error":{"type":"invalid_request","message":"unsupported diagnostic operation"}}`, false, nil
	}
	if !commandToolEnabled(policy, nativeSSHExecPolicyToolID) {
		s.recordCommandAudit(providerID, args.SessionID, "ssh.diagnostics:summary", "deny", "not_required", "policy_denied", 0, 0, ai.CommandAuditEvent{ErrorType: "policy_denied", Error: "shell tool is disabled"})
		s.emitNativeDiagnosticsOperation("policy_denied", args.SessionID, 0, "not_required", "shell tool is disabled", 0)
		return `{"error":{"type":"policy_denied","message":"SSH command execution is disabled by Command Policy"}}`, false, nil
	}

	checks := []struct {
		name    string
		command string
	}{
		{name: "os", command: "uname -a"},
		{name: "hostname", command: "hostname"},
		{name: "user", command: "whoami"},
		{name: "identity", command: "id"},
		{name: "memory", command: "free -h"},
		{name: "disk", command: "df -h"},
	}

	for _, check := range checks {
		decision, reason := evaluateCommandPolicy(policy, nativeSSHExecPolicyToolID, args.SessionID, check.command)
		if decision != commandPolicyDecisionAllow {
			status := "policy_denied"
			if decision == commandPolicyDecisionAsk {
				status = "approval_required"
			}
			s.recordCommandAudit(providerID, args.SessionID, check.command, string(decision), "not_required", status, 0, 0, ai.CommandAuditEvent{ErrorType: status, Error: strings.TrimSpace(reason)})
			s.emitNativeDiagnosticsOperation(status, args.SessionID, len(checks), "not_required", strings.TrimSpace(reason), 0)
			return marshalDiagnosticPolicyResult(args.SessionID, check.command, decision, reason), false, nil
		}
	}

	started := time.Now()
	results := make([]aiDiagnosticCheck, 0, len(checks))
	for _, check := range checks {
		result, err := s.executeSessionCommandResult(args.SessionID, check.command)
		result.Stdout = truncateAIDiagnosticOutput(result.Stdout)
		result.Stderr = truncateAIDiagnosticOutput(result.Stderr)
		status := "executed"
		if err != nil {
			status = "execution_failed"
		}
		s.recordCommandAudit(providerID, args.SessionID, check.command, "allow", "not_required", status, result.ExitCode, result.DurationMs, ai.CommandAuditEvent{ErrorType: string(result.ErrorType), Error: result.Error})
		results = append(results, aiDiagnosticCheck{Name: check.name, Command: check.command, Result: result})
	}

	payload := map[string]any{
		"status":     "completed",
		"sessionId":  args.SessionID,
		"operation":  args.Operation,
		"durationMs": time.Since(started).Milliseconds(),
		"checks":     results,
		"health":     analyzeInfrastructureHealth(results),
	}
	s.emitNativeDiagnosticsOperation("executed", args.SessionID, len(results), "not_required", "", time.Since(started).Milliseconds())
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", false, fmt.Errorf("marshal diagnostics result: %w", err)
	}
	return string(encoded), false, nil
}

func truncateAIDiagnosticOutput(value string) string {
	if len(value) <= maxAIDiagnosticOutput {
		return value
	}
	return value[:maxAIDiagnosticOutput] + "\n[output truncated by Eiksy]"
}

func marshalDiagnosticPolicyResult(sessionID, command string, decision commandPolicyDecision, reason string) string {
	status := "policy_denied"
	if decision == commandPolicyDecisionAsk {
		status = "approval_required"
	}
	payload := map[string]any{
		"status":    status,
		"sessionId": sessionID,
		"command":   command,
		"reason":    strings.TrimSpace(reason),
	}
	encoded, _ := json.Marshal(payload)
	return string(encoded)
}
