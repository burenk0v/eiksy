package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"eiksy/internal/domain/ai"
	"eiksy/internal/domain/sessions"
	"eiksy/internal/domain/workspace"
	"eiksy/internal/storage/memory"
)

type nativeTestSSHManager struct {
	mu       sync.Mutex
	commands []string
}

func (m *nativeTestSSHManager) Connect(context.Context, string, string, int, string, string, map[string]string) error { return nil }
func (m *nativeTestSSHManager) SendInput(sessionID, data string) error { m.mu.Lock(); defer m.mu.Unlock(); m.commands = append(m.commands, sessionID+":"+data); return nil }
func (m *nativeTestSSHManager) ResizeTerminal(string, int, int) error { return nil }
func (m *nativeTestSSHManager) Disconnect(string) error { return nil }
func (m *nativeTestSSHManager) SetOutputHandler(string, func(string)) {}
func (m *nativeTestSSHManager) GetCurrentDir(string) (string, error) { return ".", nil }
func (m *nativeTestSSHManager) AcceptHostKey(string) error { return nil }

func TestCallNativeToolCompletionParsesToolCall(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/chat/completions" { t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path) }
		var request map[string]any
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil { t.Fatalf("decode request: %v", err) }
		if _, ok := request["tools"]; !ok { t.Fatal("expected tools in chat completion request") }
		if request["tool_choice"] != "auto" { t.Fatalf("expected tool_choice=auto, got %#v", request["tool_choice"]) }
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"","tool_calls":[{"id":"call-1","type":"function","function":{"name":"ssh.exec","arguments":"{\"sessionId\":\"session-1\",\"command\":\"uname -a\",\"reason\":\"inspect host\"}"}}]}}]}`))
	}))
	defer server.Close()
	service := NewService(memory.NewStore(), nil, nil)
	provider := &ai.ProviderDescriptor{Model: "qwen3", Endpoint: server.URL + "/v1"}
	result, err := service.callNativeToolCompletion(context.Background(), provider, []nativeChatMessage{{Role: "user", Content: "inspect"}}, "chat-1", openAIToolDefinitions())
	if err != nil { t.Fatalf("call native completion: %v", err) }
	if len(result.ToolCalls) != 1 { t.Fatalf("expected one tool call, got %d", len(result.ToolCalls)) }
	if result.ToolCalls[0].Function.Name != nativeSSHExecToolName { t.Fatalf("expected ssh.exec, got %q", result.ToolCalls[0].Function.Name) }
}

func TestDispatchNativeToolCallRejectsUnknownArguments(t *testing.T) {
	service := NewService(memory.NewStore(), nil, nil)
	call := nativeToolCall{ID: "call-1", Type: "function"}
	call.Function.Name = nativeSSHExecToolName
	call.Function.Arguments = `{"sessionId":"session-1","command":"uname -a","unexpected":true}`
	result, pending, err := service.dispatchNativeToolCall(call, ai.CommandPolicy{Tools: []ai.CommandTool{{ID: nativeSSHExecPolicyToolID, Enabled: true}}}, "session-1", "provider-1", "inspect", nil)
	if err != nil { t.Fatalf("dispatch returned unexpected error: %v", err) }
	if pending { t.Fatal("invalid arguments must not create an approval request") }
	if result == "" || !strings.Contains(result, "invalid arguments") { t.Fatalf("expected structured invalid-arguments result, got %q", result) }
}

func TestNativeToolSystemPromptDoesNotExposeCredentials(t *testing.T) {
	service := NewService(memory.NewStore(), nil, nil)
	prompt := service.nativeToolSystemPrompt(ai.CommandPolicy{Tools: []ai.CommandTool{{ID: nativeSSHExecPolicyToolID, Enabled: true}}}, "session-1")
	if prompt == "" { t.Fatal("expected system prompt") }
	if containsAny(prompt, "password", "token", "private key", "secret") { t.Fatalf("system prompt appears to expose credential material: %q", prompt) }
}

func TestNativeToolApprovalResumesConversation(t *testing.T) {
	store := memory.NewStore()
	ssh := &nativeTestSSHManager{}
	store.OpenRuntimeTab(workspace.Tab{ID: "session-1", ProtocolID: "ssh", Status: "connected"})
	store.UpdateAIState(ai.WorkspaceState{
		Providers: []ai.ProviderDescriptor{{ID: "provider-1", Model: "qwen3", Endpoint: "", Class: ai.ProviderClassOpenAICompatible, Selected: true, Configured: true}},
		CommandPolicy: ai.CommandPolicy{Tools: []ai.CommandTool{{ID: nativeSSHExecPolicyToolID, Enabled: true}}},
		ChatSessionID: "chat-1",
	})
	var mu sync.Mutex
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct { Messages []nativeChatMessage `json:"messages"` }
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil { t.Fatalf("decode request: %v", err) }
		mu.Lock(); requestCount++; count := requestCount; mu.Unlock()
		if count == 1 {
			_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"","tool_calls":[{"id":"call-approval","type":"function","function":{"name":"ssh.exec","arguments":"{\"sessionId\":\"session-1\",\"command\":\"uname -a\",\"reason\":\"inspect host\"}"}}]}}]}`))
			return
		}
		foundToolResult := false
		for _, message := range request.Messages { if message.Role == "tool" && message.ToolCallID == "call-approval" && strings.Contains(message.Content, `"status":"executed"`) { foundToolResult = true } }
		if !foundToolResult { t.Fatalf("expected executed tool result in continuation: %#v", request.Messages) }
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"Command completed successfully."}}]}`))
	}))
	defer server.Close()
	state := store.AIState(); state.Providers[0].Endpoint = server.URL + "/v1"; store.UpdateAIState(state)
	service := NewService(store, ssh, nil)
	if _, handled, err := service.sendChatMessageWithNativeTools(context.Background(), &state.Providers[0], state, "session-1", "inspect host"); err != nil || !handled { t.Fatalf("initial native call failed: handled=%v err=%v", handled, err) }
	pending := store.AIState().PendingNativeToolCall
	if pending == nil { t.Fatal("expected pending native tool call") }
	if pending.ToolCallID != "call-approval" { t.Fatalf("unexpected pending tool call id: %q", pending.ToolCallID) }
	if err := service.ResolveCommandPolicyRequest(pending.RequestID, ai.CommandPermissionModeNow); err != nil { t.Fatalf("resolve approval: %v", err) }
	if len(ssh.commands) != 1 || ssh.commands[0] != "session-1:uname -a\n" { t.Fatalf("expected one executed command, got %#v", ssh.commands) }
	finalState := store.AIState()
	if finalState.PendingNativeToolCall != nil { t.Fatal("expected pending native tool call to be cleared") }
	if len(finalState.CommandPolicy.PendingRequests) != 0 { t.Fatalf("expected pending requests to be cleared, got %d", len(finalState.CommandPolicy.PendingRequests)) }
	if len(finalState.Messages) != 2 || finalState.Messages[0].Role != "user" || finalState.Messages[1].Content != "Command completed successfully." { t.Fatalf("unexpected final chat messages: %#v", finalState.Messages) }
}

var _ sessions.Profile
