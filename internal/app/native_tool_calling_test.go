package app

import (
    "context"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"

    "eiksy/internal/domain/ai"
    "eiksy/internal/storage/memory"
)

func TestCallNativeToolCompletionParsesToolCall(t *testing.T) {
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.Method != http.MethodPost || r.URL.Path != "/v1/chat/completions" {
            t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
        }
        var request map[string]any
        if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
            t.Fatalf("decode request: %v", err)
        }
        if _, ok := request["tools"]; !ok {
            t.Fatal("expected tools in chat completion request")
        }
        if request["tool_choice"] != "auto" {
            t.Fatalf("expected tool_choice=auto, got %#v", request["tool_choice"])
        }
        _, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"","tool_calls":[{"id":"call-1","type":"function","function":{"name":"ssh.exec","arguments":"{\\"sessionId\\":\\"session-1\\",\\"command\\":\\"uname -a\\",\\"reason\\":\\"inspect host\\"}"}}]}}]}`))
    }))
    defer server.Close()

    service := NewService(memory.NewStore(), nil, nil)
    provider := &ai.ProviderDescriptor{Model: "qwen3", Endpoint: server.URL + "/v1"}
    result, err := service.callNativeToolCompletion(context.Background(), provider, []nativeChatMessage{{Role: "user", Content: "inspect"}}, "chat-1", openAIToolDefinitions())
    if err != nil {
        t.Fatalf("call native completion: %v", err)
    }
    if len(result.ToolCalls) != 1 {
        t.Fatalf("expected one tool call, got %d", len(result.ToolCalls))
    }
    if result.ToolCalls[0].Function.Name != "ssh.exec" {
        t.Fatalf("expected ssh.exec, got %q", result.ToolCalls[0].Function.Name)
    }
}

func TestDispatchNativeToolCallRejectsUnknownArguments(t *testing.T) {
    service := NewService(memory.NewStore(), nil, nil)
    call := nativeToolCall{ID: "call-1", Type: "function"}
    call.Function.Name = "ssh.exec"
    call.Function.Arguments = `{"sessionId":"session-1","command":"uname -a","unexpected":true}`

    result, pending, err := service.dispatchNativeToolCall(call, ai.CommandPolicy{Tools: []ai.CommandTool{{ID: "shell", Enabled: true}}}, "session-1")
    if err != nil {
        t.Fatalf("dispatch returned unexpected error: %v", err)
    }
    if pending {
        t.Fatal("invalid arguments must not create an approval request")
    }
    if result == "" {
        t.Fatal("expected structured tool error result")
    }
}

func TestNativeToolSystemPromptDoesNotExposeCredentials(t *testing.T) {
    service := NewService(memory.NewStore(), nil, nil)
    prompt := service.nativeToolSystemPrompt(ai.CommandPolicy{Tools: []ai.CommandTool{{ID: "shell", Enabled: true}}}, "session-1")
    if prompt == "" {
        t.Fatal("expected system prompt")
    }
    if containsAny(prompt, "password", "token", "private key", "secret") {
        t.Fatalf("system prompt appears to expose credential material: %q", prompt)
    }
}

func containsAny(value string, terms ...string) bool {
    for _, term := range terms {
        if containsFold(value, term) {
            return true
        }
    }
    return false
}

func containsFold(value, term string) bool {
    if len(term) == 0 {
        return false
    }
    for i := 0; i+len(term) <= len(value); i++ {
        match := true
        for j := range term {
            a, b := value[i+j], term[j]
            if a >= 'A' && a <= 'Z' { a += 'a' - 'A' }
            if b >= 'A' && b <= 'Z' { b += 'a' - 'A' }
            if a != b { match = false; break }
        }
        if match { return true }
    }
    return false
}
