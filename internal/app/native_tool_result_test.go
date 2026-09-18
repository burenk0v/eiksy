package app

import (
	"context"
	"encoding/json"
	"testing"

	"eiksy/internal/domain/ai"
	"eiksy/internal/domain/sessions"
	"eiksy/internal/domain/workspace"
	"eiksy/internal/storage/memory"
)

type structuredToolSSHManager struct{}

func (m *structuredToolSSHManager) Connect(context.Context, string, string, int, string, string, map[string]string) error {
	return nil
}
func (m *structuredToolSSHManager) SendInput(string, string) error { return nil }
func (m *structuredToolSSHManager) ResizeTerminal(string, int, int) error { return nil }
func (m *structuredToolSSHManager) Disconnect(string) error { return nil }
func (m *structuredToolSSHManager) SetOutputHandler(string, func(string)) {}
func (m *structuredToolSSHManager) GetCurrentDir(string) (string, error) { return ".", nil }
func (m *structuredToolSSHManager) AcceptHostKey(string) error { return nil }
func (m *structuredToolSSHManager) ExecCommandResult(context.Context, string, string) (sessions.CommandExecutionResult, error) {
	return sessions.CommandExecutionResult{
		Success:    true,
		ExitCode:   0,
		Stdout:     "Linux eiksy-test",
		Stderr:     "",
		DurationMs: 17,
	}, nil
}

func TestDispatchNativeToolCallReturnsStructuredExecutionResult(t *testing.T) {
	store := memory.NewStore()
	store.OpenRuntimeTab(workspace.Tab{ID: "session-1", ProtocolID: "ssh", Status: "connected"})
	service := NewService(store, &structuredToolSSHManager{}, nil)

	call := nativeToolCall{ID: "call-1", Type: "function"}
	call.Function.Name = nativeSSHExecToolName
	call.Function.Arguments = `{"sessionId":"session-1","command":"uname -a","reason":"inspect host"}`

	policy := ai.CommandPolicy{
		Tools:        []ai.CommandTool{{ID: nativeSSHExecPolicyToolID, Enabled: true}},
		CommandRules: []ai.CommandRule{{ToolID: nativeSSHExecPolicyToolID, Pattern: "uname -a", Action: ai.CommandPermissionAllow}},
	}
	result, pending, err := service.dispatchNativeToolCall(call, policy, "session-1", "provider-1", "inspect host", nil)
	if err != nil {
		t.Fatalf("dispatch failed: %v", err)
	}
	if pending {
		t.Fatal("allow decision must execute immediately")
	}

	var payload struct {
		Status    string `json:"status"`
		SessionID string `json:"sessionId"`
		Command   string `json:"command"`
		Result    sessions.CommandExecutionResult `json:"result"`
	}
	if err := json.Unmarshal([]byte(result), &payload); err != nil {
		t.Fatalf("decode tool result: %v; payload=%s", err, result)
	}
	if payload.Status != "executed" || payload.SessionID != "session-1" || payload.Command != "uname -a" {
		t.Fatalf("unexpected execution envelope: %+v", payload)
	}
	if !payload.Result.Success || payload.Result.ExitCode != 0 || payload.Result.Stdout != "Linux eiksy-test" || payload.Result.DurationMs != 17 {
		t.Fatalf("unexpected structured result: %+v", payload.Result)
	}
}
