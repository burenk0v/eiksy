package app

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "strings"
    "time"

    "eiksy/internal/domain/ai"
)

type nativeChatMessage struct {
    Role       string                 `json:"role"`
    Content    string                 `json:"content,omitempty"`
    ToolCallID string                 `json:"tool_call_id,omitempty"`
    ToolCalls  []nativeToolCall       `json:"tool_calls,omitempty"`
}

type nativeToolCall struct {
    ID       string `json:"id"`
    Type     string `json:"type"`
    Function struct {
        Name      string `json:"name"`
        Arguments string `json:"arguments"`
    } `json:"function"`
}

type nativeChatResponse struct {
    Choices []struct {
        Message nativeChatMessage `json:"message"`
    } `json:"choices"`
}

// sendChatMessageWithNativeTools is the native OpenAI-compatible tool-calling
// path. It is deliberately isolated from the legacy text-command path so the
// dispatcher can be tested and evolved independently.
func (s *Service) sendChatMessageWithNativeTools(ctx context.Context, provider *ai.ProviderDescriptor, state ai.WorkspaceState, activeSessionID, userMessage string) (string, bool, error) {
    if provider == nil {
        return "", false, nil
    }

    messages := []nativeChatMessage{{
        Role: "system",
        Content: s.nativeToolSystemPrompt(state.CommandPolicy, activeSessionID),
    }}
    for _, message := range state.Messages {
        messages = append(messages, nativeChatMessage{Role: message.Role, Content: message.Content})
    }
    messages = append(messages, nativeChatMessage{Role: "user", Content: userMessage})

    toolPolicy := normalizeCommandPolicy(state.CommandPolicy)
    if !commandToolEnabled(toolPolicy, "shell") {
        return "", false, nil
    }

    tools := openAIToolDefinitions()
    for turn := 0; turn < 4; turn++ {
        response, err := s.callNativeToolCompletion(ctx, provider, messages, state.ChatSessionID, tools)
        if err != nil {
            return "", true, err
        }
        if len(response.ToolCalls) == 0 {
            reply := strings.TrimSpace(response.Content)
            if reply == "" {
                return "", true, fmt.Errorf("AI returned an empty response")
            }
            latest := s.store.AIState()
            latest.Messages = append(latest.Messages, ai.ChatMessage{Role: "user", Content: userMessage})
            latest.Messages = append(latest.Messages, ai.ChatMessage{Role: "assistant", Content: reply})
            s.store.UpdateAIState(latest)
            s.emitFn("ai:message", map[string]string{"role": "assistant", "content": reply})
            return reply, true, nil
        }

        assistant := nativeChatMessage{Role: "assistant", Content: response.Content, ToolCalls: response.ToolCalls}
        messages = append(messages, assistant)

        for _, call := range response.ToolCalls {
            result, pending, err := s.dispatchNativeToolCall(call, toolPolicy, activeSessionID)
            if err != nil {
                return "", true, err
            }
            if pending {
                return result, true, nil
            }
            messages = append(messages, nativeChatMessage{
                Role:       "tool",
                ToolCallID: call.ID,
                Content:    result,
            })
        }
    }

    return "", true, fmt.Errorf("AI exceeded the maximum number of tool-calling turns")
}

func (s *Service) callNativeToolCompletion(ctx context.Context, provider *ai.ProviderDescriptor, messages []nativeChatMessage, chatSessionID string, tools []map[string]any) (nativeChatMessage, error) {
    body, err := json.Marshal(map[string]any{
        "model":             provider.Model,
        "messages":          messages,
        "tools":             tools,
        "tool_choice":       "auto",
        "parallel_tool_calls": false,
        "user":               chatSessionID,
    })
    if err != nil {
        return nativeChatMessage{}, err
    }

    endpoint := strings.TrimRight(provider.Endpoint, "/") + "/chat/completions"
    request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
    if err != nil {
        return nativeChatMessage{}, err
    }
    request.Header.Set("Content-Type", "application/json")
    if strings.TrimSpace(provider.Token) != "" {
        request.Header.Set("Authorization", "Bearer "+provider.Token)
    }

    response, err := s.httpClient.Do(request)
    if err != nil {
        return nativeChatMessage{}, err
    }
    defer response.Body.Close()

    var payload nativeChatResponse
    if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
        return nativeChatMessage{}, fmt.Errorf("parse native AI response: %w", err)
    }
    if response.StatusCode != http.StatusOK {
        return nativeChatMessage{}, fmt.Errorf("AI API returned %s", response.Status)
    }
    if len(payload.Choices) == 0 {
        return nativeChatMessage{}, fmt.Errorf("AI returned no choices")
    }
    return payload.Choices[0].Message, nil
}

func (s *Service) dispatchNativeToolCall(call nativeToolCall, policy ai.CommandPolicy, activeSessionID string) (string, bool, error) {
    if call.Type != "function" && call.Type != "" {
        return "", false, fmt.Errorf("unsupported tool call type %q", call.Type)
    }
    if call.Function.Name != "ssh.exec" {
        return fmt.Sprintf(`{"error":"unknown tool %q"}`, call.Function.Name), false, nil
    }

    var args struct {
        SessionID string `json:"sessionId"`
        Command   string `json:"command"`
        Reason    string `json:"reason"`
    }
    decoder := json.NewDecoder(strings.NewReader(call.Function.Arguments))
    decoder.DisallowUnknownFields()
    if err := decoder.Decode(&args); err != nil {
        return fmt.Sprintf(`{"error":"invalid arguments: %s"}`, err), false, nil
    }
    args.SessionID = strings.TrimSpace(args.SessionID)
    args.Command = strings.TrimSpace(args.Command)
    args.Reason = strings.TrimSpace(args.Reason)
    if args.SessionID == "" {
        args.SessionID = strings.TrimSpace(activeSessionID)
    }
    if args.SessionID == "" || args.Command == "" {
        return `{"error":"sessionId and command are required"}`, false, nil
    }

    decision, reason := evaluateCommandPolicy(policy, "shell", args.SessionID, args.Command)
    switch decision {
    case commandPolicyDecisionDeny:
        result := fmt.Sprintf(`{"error":"command denied by Command Policy"}`)
        if strings.TrimSpace(reason) != "" {
            result = fmt.Sprintf(`{"error":"command denied by Command Policy","reason":%q}`, reason)
        }
        return result, false, nil
    case commandPolicyDecisionAllow:
        if err := s.executeSessionCommand(args.SessionID, args.Command); err != nil {
            return fmt.Sprintf(`{"error":"command execution failed","message":%q}`, err.Error()), false, nil
        }
        return fmt.Sprintf(`{"ok":true,"sessionId":%q,"command":%q}`, args.SessionID, args.Command), false, nil
    case commandPolicyDecisionAsk:
        state := s.store.AIState()
        state.CommandPolicy = normalizeCommandPolicy(state.CommandPolicy)
        request := ai.CommandRequest{
            ID:          fmt.Sprintf("cmdreq-%d", time.Now().UTC().UnixNano()),
            ToolID:      "shell",
            SessionID:   args.SessionID,
            Command:     args.Command,
            Reason:      args.Reason,
            RequestedAt: time.Now().UTC().Format(time.RFC3339),
        }
        state.CommandPolicy.PendingRequests = append(state.CommandPolicy.PendingRequests, request)
        state.Messages = append(state.Messages, ai.ChatMessage{Role: "user", Content: ""})
        state.Messages = append(state.Messages, ai.ChatMessage{Role: "assistant", Content: fmt.Sprintf("Command permission required for session %s.\nCommand: `%s`\nReason: %s", request.SessionID, request.Command, request.Reason)})
        s.store.UpdateAIState(state)
        s.emitFn("ai:message", map[string]string{"role": "assistant", "content": fmt.Sprintf("Command permission required for session %s.\nCommand: `%s`\nReason: %s", request.SessionID, request.Command, request.Reason)})
        return fmt.Sprintf(`{"status":"approval_required","requestId":%q}`, request.ID), true, nil
    default:
        return `{"error":"unknown policy decision"}`, false, nil
    }
}

func (s *Service) nativeToolSystemPrompt(policy ai.CommandPolicy, activeSessionID string) string {
    normalized := normalizeCommandPolicy(policy)
    enabled := make([]string, 0, len(normalized.Tools))
    for _, tool := range normalized.Tools {
        if tool.Enabled {
            enabled = append(enabled, tool.ID)
        }
    }
    return strings.TrimSpace(fmt.Sprintf(
        "You are connected to Eiksy. Use registered tools when an action is required. Never invent tools. The ssh.exec tool executes exactly one command in an active SSH session and is always enforced by Command Policy. Active session: %q. Enabled policy tools: [%s].",
        activeSessionID, strings.Join(enabled, ", "),
    ))
}
