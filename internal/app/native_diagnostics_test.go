package app

import (
	"encoding/json"
	"strings"
	"testing"

	"eiksy/internal/domain/ai"
	"eiksy/internal/domain/workspace"
	"eiksy/internal/storage/memory"
)

func TestDispatchNativeDiagnosticsSummaryUsesFixedCommands(t *testing.T) {
	store := memory.NewStore()
	ssh := &nativeTestSSHManager{}
	store.OpenRuntimeTab(workspace.Tab{ID: "session-1", ProtocolID: "ssh", Status: "connected"})
	service := NewService(store, ssh, nil)
	policy := ai.CommandPolicy{Tools: []ai.CommandTool{{ID: nativeSSHExecPolicyToolID, Enabled: true}}, CommandRules: defaultCommandRules()}

	call := nativeToolCall{ID: "diag-1", Type: "function"}
	call.Function.Name = nativeSSHDiagnosticsToolName
	call.Function.Arguments = `{"sessionId":"session-1","operation":"summary"}`

	result, pending, err := service.dispatchNativeDiagnostics(call, policy, "session-1", "provider-1")
	if err != nil {
		t.Fatalf("dispatch diagnostics: %v", err)
	}
	if pending {
		t.Fatal("diagnostics must not create an approval request")
	}

	var payload struct {
		Status    string `json:"status"`
		SessionID string `json:"sessionId"`
		Operation string `json:"operation"`
		Checks    []struct {
			Name   string `json:"name"`
			Command string `json:"command"`
		} `json:"checks"`
	}
	if err := json.Unmarshal([]byte(result), &payload); err != nil {
		t.Fatalf("decode diagnostics result: %v; result=%s", err, result)
	}
	if payload.Status != "completed" || payload.SessionID != "session-1" || payload.Operation != "summary" {
		t.Fatalf("unexpected diagnostics envelope: %#v", payload)
	}
	if len(payload.Checks) != 6 {
		t.Fatalf("expected six fixed checks, got %d", len(payload.Checks))
	}
	for _, check := range payload.Checks {
		if strings.Contains(check.Command, ";") || strings.Contains(check.Command, "|") || strings.Contains(check.Command, "$") {
			t.Fatalf("diagnostic command contains shell composition: %q", check.Command)
		}
	}
	if len(ssh.commands) != 6 {
		t.Fatalf("expected six SSH executions, got %#v", ssh.commands)
	}
}

func TestDispatchNativeDiagnosticsRejectsUnsupportedOperation(t *testing.T) {
	service := NewService(memory.NewStore(), &nativeTestSSHManager{}, nil)
	call := nativeToolCall{ID: "diag-1", Type: "function"}
	call.Function.Name = nativeSSHDiagnosticsToolName
	call.Function.Arguments = `{"sessionId":"session-1","operation":"custom"}`

	result, pending, err := service.dispatchNativeDiagnostics(call, ai.CommandPolicy{}, "session-1", "provider-1")
	if err != nil {
		t.Fatalf("dispatch returned unexpected error: %v", err)
	}
	if pending {
		t.Fatal("unsupported operation must not create an approval request")
	}
	if !strings.Contains(result, "unsupported diagnostic operation") {
		t.Fatalf("expected unsupported-operation error, got %q", result)
	}
}

func TestDispatchNativeDiagnosticsHonorsCommandPolicy(t *testing.T) {
	store := memory.NewStore()
	ssh := &nativeTestSSHManager{}
	store.OpenRuntimeTab(workspace.Tab{ID: "session-1", ProtocolID: "ssh", Status: "connected"})
	service := NewService(store, ssh, nil)
	policy := ai.CommandPolicy{Tools: []ai.CommandTool{{ID: nativeSSHExecPolicyToolID, Enabled: false}}}

	call := nativeToolCall{ID: "diag-1", Type: "function"}
	call.Function.Name = nativeSSHDiagnosticsToolName
	call.Function.Arguments = `{"sessionId":"session-1","operation":"summary"}`

	result, pending, err := service.dispatchNativeDiagnostics(call, policy, "session-1", "provider-1")
	if err != nil {
		t.Fatalf("dispatch returned unexpected error: %v", err)
	}
	if pending {
		t.Fatal("policy denial must not create an approval request")
	}
	if !strings.Contains(result, "policy_denied") {
		t.Fatalf("expected policy denial, got %q", result)
	}
	if len(ssh.commands) != 0 {
		t.Fatalf("diagnostics must not execute when policy denies the tool: %#v", ssh.commands)
	}
}
