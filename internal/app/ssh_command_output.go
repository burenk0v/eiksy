package app

import (
	"context"
	"fmt"
	"strings"

	"eiksy/internal/domain/sessions"
)

const maxAICommandOutput = 32 * 1024

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
// The SSH manager implementation enforces its own execution timeout.
func (s *Service) executeSessionCommandResult(sessionID, command string) (sessions.CommandExecutionResult, error) {
	sessionID = strings.TrimSpace(sessionID)
	command = strings.TrimSpace(command)
	if sessionID == "" {
		return sessions.CommandExecutionResult{ExitCode: -1}, fmt.Errorf("session id is required")
	}
	if command == "" {
		return sessions.CommandExecutionResult{ExitCode: -1}, fmt.Errorf("command cannot be empty")
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

		if executor, ok := s.sshManager.(sshStructuredCommandExecutor); ok {
			return executor.ExecCommandResult(s.resolveContext(s.ctx), sessionID, command)
		}
		if executor, ok := s.sshManager.(sshCommandExecutor); ok {
			output, err := executor.ExecCommand(sessionID, command)
			result := sessions.CommandExecutionResult{Success: err == nil, ExitCode: 0, Stdout: output}
			if err != nil {
				result.Success = false
				result.ExitCode = -1
				result.Error = err.Error()
			}
			return result, err
		}
		return sessions.CommandExecutionResult{ExitCode: -1}, fmt.Errorf("ssh manager does not support command execution")
	}
	return sessions.CommandExecutionResult{ExitCode: -1}, fmt.Errorf("active ssh session %q not found", sessionID)
}
