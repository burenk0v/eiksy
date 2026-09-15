package sshmanager

import (
	"fmt"
	"strings"
)

const maxExecOutput = 32 * 1024

// ExecCommand executes one non-interactive command over the existing SSH
// connection. It uses a fresh SSH exec session, so command execution does not
// mutate the state of the interactive terminal session.
func (m *Manager) ExecCommand(tabID, command string) (string, error) {
	tabID = strings.TrimSpace(tabID)
	command = strings.TrimSpace(command)
	if tabID == "" {
		return "", fmt.Errorf("ssh tab id is required")
	}
	if command == "" {
		return "", fmt.Errorf("command cannot be empty")
	}

	m.mu.RLock()
	conn := m.connections[tabID]
	m.mu.RUnlock()
	if conn == nil || conn.client == nil {
		return "", fmt.Errorf("ssh tab %q is not connected", tabID)
	}

	session, err := conn.client.NewSession()
	if err != nil {
		return "", fmt.Errorf("create ssh exec session: %w", err)
	}
	defer session.Close()

	output, err := session.CombinedOutput(command)
	result := string(output)
	if len(result) > maxExecOutput {
		result = result[:maxExecOutput] + "\n[output truncated by Eiksy]"
	}
	if err != nil {
		return result, fmt.Errorf("run command over ssh: %w", err)
	}
	return result, nil
}

var _ interface {
	ExecCommand(string, string) (string, error)
} = (*Manager)(nil)
