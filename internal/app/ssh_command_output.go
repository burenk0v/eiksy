package app

import (
	"fmt"
	"strings"
)

const maxAICommandOutput = 32 * 1024

type sshCommandExecutor interface {
	ExecCommand(string, string) (string, error)
}

// executeSessionCommandWithOutput is the native-tool execution path. The
// caller must already have evaluated Command Policy before invoking it.
func (s *Service) executeSessionCommandWithOutput(sessionID, command string) (string, error) {
	sessionID = strings.TrimSpace(sessionID)
	command = strings.TrimSpace(command)
	if sessionID == "" {
		return "", fmt.Errorf("session id is required")
	}
	if command == "" {
		return "", fmt.Errorf("command cannot be empty")
	}
	if s.sshManager == nil {
		return "", fmt.Errorf("ssh manager is not configured")
	}

	for _, tab := range s.store.RuntimeTabs() {
		if tab.ID != sessionID {
			continue
		}
		if strings.ToLower(strings.TrimSpace(tab.ProtocolID)) != "ssh" {
			return "", fmt.Errorf("session %q is not an ssh session", sessionID)
		}
		executor, ok := s.sshManager.(sshCommandExecutor)
		if !ok {
			return "", fmt.Errorf("ssh manager does not support command execution")
		}
		output, err := executor.ExecCommand(sessionID, command)
		if len(output) > maxAICommandOutput {
			output = output[:maxAICommandOutput] + "\n[output truncated by Eiksy]"
		}
		return output, err
	}
	return "", fmt.Errorf("active ssh session %q not found", sessionID)
}
