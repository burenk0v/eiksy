package sshmanager

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"eiksy/internal/domain/sessions"
	xssh "golang.org/x/crypto/ssh"
)

const (
	maxExecOutput       = 32 * 1024
	execCommandTimeout  = 30 * time.Second
)

func (m *Manager) ExecCommandResult(ctx context.Context, tabID, command string) (sessions.CommandExecutionResult, error) {
	started := time.Now()
	result := sessions.CommandExecutionResult{ExitCode: -1}
	finish := func(err error, errorType sessions.CommandExecutionErrorType) (sessions.CommandExecutionResult, error) {
		result.Success = err == nil
		result.DurationMs = time.Since(started).Milliseconds()
		result.ErrorType = errorType
		if err != nil {
			result.Error = err.Error()
		}
		return result, err
	}

	tabID = strings.TrimSpace(tabID)
	command = strings.TrimSpace(command)
	if tabID == "" {
		return finish(fmt.Errorf("ssh tab id is required"), sessions.CommandExecutionErrorInvalid)
	}
	if command == "" {
		return finish(fmt.Errorf("command cannot be empty"), sessions.CommandExecutionErrorInvalid)
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, execCommandTimeout)
	defer cancel()

	m.mu.RLock()
	conn := m.connections[tabID]
	m.mu.RUnlock()
	if conn == nil || conn.client == nil {
		return finish(fmt.Errorf("ssh tab %q is not connected", tabID), sessions.CommandExecutionErrorConnection)
	}

	session, err := conn.client.NewSession()
	if err != nil {
		return finish(fmt.Errorf("create ssh exec session: %w", err), sessions.CommandExecutionErrorConnection)
	}
	defer session.Close()

	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr
	if err := session.Start(command); err != nil {
		return finish(fmt.Errorf("start command over ssh: %w", err), sessions.CommandExecutionErrorConnection)
	}

	waitCh := make(chan error, 1)
	go func() { waitCh <- session.Wait() }()

	var waitErr error
	errorType := sessions.CommandExecutionErrorInternal
	select {
	case waitErr = <-waitCh:
	case <-ctx.Done():
		_ = session.Close()
		waitErr = fmt.Errorf("command execution timed out after %s: %w", execCommandTimeout, ctx.Err())
		errorType = sessions.CommandExecutionErrorTimeout
	}

	result.Stdout = truncateExecOutput(stdout.String())
	result.Stderr = truncateExecOutput(stderr.String())
	if waitErr == nil {
		result.ExitCode = 0
		return finish(nil, "")
	}

	var exitErr *xssh.ExitError
	if errors.As(waitErr, &exitErr) {
		result.ExitCode = exitErr.ExitStatus()
		errorType = sessions.CommandExecutionErrorNonZero
	}
	return finish(fmt.Errorf("run command over ssh: %w", waitErr), errorType)
}

func truncateExecOutput(value string) string {
	if len(value) <= maxExecOutput {
		return value
	}
	return value[:maxExecOutput] + "\n[output truncated by Eiksy]"
}

func combinedExecOutput(result sessions.CommandExecutionResult) string {
	parts := make([]string, 0, 2)
	if result.Stdout != "" {
		parts = append(parts, result.Stdout)
	}
	if result.Stderr != "" {
		parts = append(parts, result.Stderr)
	}
	return strings.Join(parts, "\n")
}

var _ interface {
	ExecCommand(string, string) (string, error)
} = (*Manager)(nil)

var _ interface {
	ExecCommandResult(context.Context, string, string) (sessions.CommandExecutionResult, error)
} = (*Manager)(nil)
