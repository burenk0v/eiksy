package app

import (
	"context"
	"fmt"
	"strings"

	"eiksy/internal/domain/sessions"
)

const maxAICommandOutput = 32 * 1024
const maxAICommandLength = 8 * 1024
const maxAICommandReasonLength = 2 * 1024

type sshCommandExecutor interface {
	ExecCommand(string, string) (string, error)
}

type sshStructuredCommandExecutor interface {
	ExecCommandResult(context.Context, string, string) (sessions.CommandExecutionResult, error)
}

// executeSessionCommandWithOutput is the legacy text-oriented execution path.
// Native tool calling uses executeSessionCommandResult below so stdout,
// stderr, exit code and duration remain machine-readable.
func (s *Service) executeSessionCommandWithOutput(sessionID, command string) (string, error) {
	result, err := s.executeSessionCommandResult(sessionID, command)
	output := result.Stdout
	if result.Stderr != "" {
		if output != "" {
			output += "\n"
		}
		output += result.Stderr
	}
	if len(output) > maxAICommandOutput {
		output = output[:maxAICommandOutput] + "\n[output truncated by Eiksy]"
	}
	return output, err
}

// executeSessionCommandResult is the structured execution boundary used by AI.
// The application boundary enforces input/output limits even when an SSH
// manager implementation does not enforce them itself.
func (s *Service) executeSessionCommandResult(sessionID, command string) (sessions.CommandExecutionResult, error) {
	sessionID = strings.TrimSpace(sessionID)
	command = strings.TrimSpace(command)
	if sessionID == "" {
		return sessions.CommandExecutionResult{ExitCode: -1}, fmt.Errorf("session id is required")
	}
	if command == "" {
		return sessions.CommandExecutionResult{ExitCode: -1}, fmt.Errorf("command cannot be empty")
	}
	if len(command) > maxAICommandLength {
		return sessions.CommandExecutionResult{ExitCode: -1, ErrorType: sessions.CommandExecutionErrorInvalid}, fmt.Errorf("command exceeds the maximum length of %d bytes", maxAICommandLength)
	}
	if s.sshManager == nil {
		return sessions.CommandExecutionResult{ExitCode: -1}, fmt.Errorf("ssh manager is not configured")
	}

	for _, tab := range s.store.RuntimeTabs() {
		if tab.ID != sessionID {
			continue
		}
		if strings.ToLower(strings.TrimSpace(tab.ProtocolID)) != "ssh" {
			return sessions.CommandExecutionResult{ExitCode: -1}, fmt.Errorf("session %q is not an ssh session", sessionID)
		}

		var result sessions.CommandExecutionResult
		var err error
		if executor, ok := s.sshManager.(sshStructuredCommandExecutor); ok {
			result, err = executor.ExecCommandResult(s.resolveContext(s.ctx), sessionID, command)
		} else if executor, ok := s.sshManager.(sshCommandExecutor); ok {
			output, execErr := executor.ExecCommand(sessionID, command)
			result = sessions.CommandExecutionResult{Success: execErr == nil, ExitCode: 0, Stdout: output}
			err = execErr
			if execErr != nil {
				result.Success = false
				result.ExitCode = -1
				result.Error = execErr.Error()
			}
		} else {
			return sessions.CommandExecutionResult{ExitCode: -1}, fmt.Errorf("ssh manager does not support command execution")
		}

		return boundAICommandResult(result), err
	}
	return sessions.CommandExecutionResult{ExitCode: -1}, fmt.Errorf("active ssh session %q not found", sessionID)
}

// boundAICommandResult is the final application boundary before a command
// result can enter the AI conversation or UI. It protects callers that provide
// an SSH manager with a larger result than the built-in implementation allows.
func boundAICommandResult(result sessions.CommandExecutionResult) sessions.CommandExecutionResult {
	result.Stdout = truncateAICommandOutput(result.Stdout)
	result.Stderr = truncateAICommandOutput(result.Stderr)
	result.Error = truncateAICommandOutput(result.Error)
	return result
}

func truncateAICommandOutput(value string) string {
	if len(value) <= maxAICommandOutput {
		return value
	}
	return value[:maxAICommandOutput] + "\n[output truncated by Eiksy]"
}

func truncateAICommandReason(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= maxAICommandReasonLength {
		return value
	}
	return value[:maxAICommandReasonLength] + "...[truncated by Eiksy]"
}
