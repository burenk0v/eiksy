package app

import (
	"context"
	"fmt"
	"strings"

	"eiksy/internal/domain/sessions"
)

type sshStructuredCommandExecutor interface {
	ExecCommandResult(context.Context, string, string) (sessions.CommandExecutionResult, error)
}

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
		executor, ok := s.sshManager.(sshStructuredCommandExecutor)
		if !ok {
			return sessions.CommandExecutionResult{ExitCode: -1}, fmt.Errorf("ssh manager does not support structured command execution")
		}
		return executor.ExecCommandResult(s.resolveContext(s.ctx), sessionID, command)
	}
	return sessions.CommandExecutionResult{ExitCode: -1}, fmt.Errorf("active ssh session %q not found", sessionID)
}
