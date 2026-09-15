package app

import (
	"context"
	"strings"
	"testing"

	"eiksy/internal/domain/sessions"
	"eiksy/internal/domain/workspace"
	"eiksy/internal/storage/memory"
)

type outputSSHManager struct {
	output string
	err    error
}

func (m *outputSSHManager) Connect(context.Context, string, string, int, string, string, map[string]string) error { return nil }
func (m *outputSSHManager) SendInput(string, string) error { return nil }
func (m *outputSSHManager) ResizeTerminal(string, int, int) error { return nil }
func (m *outputSSHManager) Disconnect(string) error { return nil }
func (m *outputSSHManager) SetOutputHandler(string, func(string)) {}
func (m *outputSSHManager) GetCurrentDir(string) (string, error) { return ".", nil }
func (m *outputSSHManager) AcceptHostKey(string) error { return nil }
func (m *outputSSHManager) ExecCommand(string, string) (string, error) { return m.output, m.err }

func TestExecuteSessionCommandWithOutput(t *testing.T) {
	store := memory.NewStore()
	store.OpenRuntimeTab(workspace.Tab{ID: "ssh-1", ProtocolID: "ssh", ProfileID: "p"})
	manager := &outputSSHManager{output: "stdout\nstderr\n"}
	service := NewService(store, manager, nil)

	output, err := service.executeSessionCommandWithOutput("ssh-1", "printf test")
	if err != nil { t.Fatalf("execute command: %v", err) }
	if output != "stdout\nstderr\n" { t.Fatalf("unexpected output: %q", output) }
}

func TestExecuteSessionCommandWithOutputReturnsErrorAndOutput(t *testing.T) {
	store := memory.NewStore()
	store.OpenRuntimeTab(workspace.Tab{ID: "ssh-1", ProtocolID: "ssh", ProfileID: "p"})
	manager := &outputSSHManager{output: "partial output", err: context.Canceled}
	service := NewService(store, manager, nil)

	output, err := service.executeSessionCommandWithOutput("ssh-1", "false")
	if err == nil { t.Fatal("expected command execution error") }
	if output != "partial output" { t.Fatalf("expected partial output, got %q", output) }
}

func TestExecuteSessionCommandWithOutputTruncatesLargeOutput(t *testing.T) {
	store := memory.NewStore()
	store.OpenRuntimeTab(workspace.Tab{ID: "ssh-1", ProtocolID: "ssh", ProfileID: "p"})
	manager := &outputSSHManager{output: strings.Repeat("x", maxAICommandOutput+1024)}
	service := NewService(store, manager, nil)

	output, err := service.executeSessionCommandWithOutput("ssh-1", "yes")
	if err != nil { t.Fatalf("execute command: %v", err) }
	if !strings.HasSuffix(output, "\n[output truncated by Eiksy]") { t.Fatalf("expected truncation marker") }
	if len(output) <= maxAICommandOutput { t.Fatalf("expected marker to extend beyond output limit, got %d bytes", len(output)) }
}

var _ sessions.Profile
