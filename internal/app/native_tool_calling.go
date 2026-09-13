package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"eiksy/internal/domain/ai"
	"eiksy/internal/securestorage"
)

const nativeSSHExecToolName = "ssh.exec"
const nativeSSHExecPolicyToolID = "shell"

type nativeChatMessage struct {
	Role       string           `json:"role"`
	Content    string           `json:"content,omitempty"`
	ToolCallID string           `json:"tool_call_id,omitempty"`
	ToolCalls  []nativeToolCall `json:"tool_calls,omitempty"`
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

func (s *Service) sendChatMessageWithNativeTools(ctx context.Context, provider *ai.ProviderDescriptor, state ai.WorkspaceState, activeSessionID, userMessage string) (string, bool, error) {
	if provider == nil || !commandToolEnabled(state.CommandPolicy, nativeSSHExecPolicyToolID) {
		return "", false, nil
	}

	messages := s.nativeMessagesFromState(state, activeSessionID)
	messages = append(messages, nativeChatMessage{Role: "user", Content: userMessage})
	return s.runNativeToolLoop(ctx, provider, state.ChatSessionID, state.CommandPolicy, activeSessionID, userMessage, messages)
}

func (s *Service) runNativeToolLoop(ctx context.Context, provider *ai.ProviderDescriptor, chatSessionID string, policy ai.CommandPolicy, activeSessionID, userMessage string, messages []nativeChatMessage) (string, bool, error) {
	toolPolicy := normalizeCommandPolicy(policy)
	tools := openAIToolDefinitions()

	for turn := 0; turn < 4; turn++ {
		response, err := s.callNativeToolCompletion(ctx, provider, messages, chatSessionID, tools)
		if err != nil {
			return "", true, err
		}
		if len(response.ToolCalls) == 0 {
			reply := strings.TrimSpace(response.Content)
			if reply == "" {
				return "", true, fmt.Errorf("AI returned an empty response")
			}
			latest := s.store.AIState()
			latest.PendingNativeToolCall = nil
			latest.Messages = append(latest.Messages, ai.ChatMessage{Role: "user", Content: userMessage})
			latest.Messages = append(latest.Messages, ai.ChatMessage{Role: "assistant", Content: reply})
			s.store.UpdateAIState(latest)
			s.emitFn("ai:message", map[string]string{"role": "assistant", "content": reply})
			return reply, true, nil
		}

		messages = append(messages, nativeChatMessage{Role: "assistant", Content: response.Content, ToolCalls: response.ToolCalls})
		for _, call := range response.ToolCalls {
			result, pending, err := s.dispatchNativeToolCall(call, toolPolicy, activeSessionID, provider.ID, userMessage, messages)
			if err != nil {
				return "", true, err
			}
			if pending {
				return result, true, nil
			}
			messages = append(messages, nativeChatMessage{Role: "tool", ToolCallID: call.ID, Content: result})
		}
	}

	return "", true, fmt.Errorf("AI exceeded the maximum number of tool-calling turns")
}

func (s *Service) nativeMessagesFromState(state ai.WorkspaceState, activeSessionID string) []nativeChatMessage {
	messages := []nativeChatMessage{{Role: "system", Content: s.nativeToolSystemPrompt(state.CommandPolicy, activeSessionID)}}
	for _, message := range state.Messages {
		messages = append(messages, nativeChatMessage{Role: message.Role, Content: message.Content})
	}
	return messages
}

func (s *Service) callNativeToolCompletion(ctx context.Context, provider *ai.ProviderDescriptor, messages []nativeChatMessage, chatSessionID string, tools []map[string]any) (nativeChatMessage, error) {
	body, err := json.Marshal(map[string]any{
		"model":               provider.Model,
		"messages":            messages,
		"tools":               tools,
		"tool_choice":         "auto",
		"parallel_tool_calls": false,
		"user":                chatSessionID,
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
	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return nativeChatMessage{}, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nativeChatMessage{}, fmt.Errorf("AI API returned %s: %s", response.Status, strings.TrimSpace(string(responseBody)))
	}

	var payload nativeChatResponse
	if err := json.Unmarshal(responseBody, &payload); err != nil {
		return nativeChatMessage{}, fmt.Errorf("parse native AI response: %w", err)
	}
	if len(payload.Choices) == 0 {
		return nativeChatMessage{}, fmt.Errorf("AI returned no choices")
	}
	return payload.Choices[0].Message, nil
}

func (s *Service) dispatchNativeToolCall(call nativeToolCall, policy ai.CommandPolicy, activeSessionID, providerID, userMessage string, messages []nativeChatMessage) (string, bool, error) {
	if call.Type != "function" && call.Type != "" {
		return "", false, fmt.Errorf("unsupported tool call type %q", call.Type)
	}
	if call.Function.Name != nativeSSHExecToolName {
		return marshalNativeToolError("unknown tool %q", call.Function.Name), false, nil
	}

	var args struct {
		SessionID string `json:"sessionId"`
		Command   string `json:"command"`
		Reason    string `json:"reason"`
	}
	decoder := json.NewDecoder(strings.NewReader(call.Function.Arguments))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&args); err != nil {
		return marshalNativeToolError("invalid arguments: %v", err), false, nil
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

	decision, reason := evaluateCommandPolicy(policy, nativeSSHExecPolicyToolID, args.SessionID, args.Command)
	switch decision {
	case commandPolicyDecisionDeny:
		if strings.TrimSpace(reason) == "" {
			return `{"error":"command denied by Command Policy"}`, false, nil
		}
		return fmt.Sprintf(`{"error":"command denied by Command Policy","reason":%q}`, reason), false, nil
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
			ToolID:      nativeSSHExecPolicyToolID,
			SessionID:   args.SessionID,
			Command:     args.Command,
			Reason:      args.Reason,
			RequestedAt: time.Now().UTC().Format(time.RFC3339),
		}
		encodedMessages, err := json.Marshal(messages)
		if err != nil {
			return "", false, fmt.Errorf("save pending native tool call: %w", err)
		}
		state.CommandPolicy.PendingRequests = append(state.CommandPolicy.PendingRequests, request)
		state.PendingNativeToolCall = &ai.PendingNativeToolCall{
			RequestID:     request.ID,
			ProviderID:    providerID,
			ToolCallID:    call.ID,
			ToolName:      call.Function.Name,
			ToolArguments: call.Function.Arguments,
			UserMessage:   userMessage,
			SessionID:     args.SessionID,
			MessagesJSON:  string(encodedMessages),
		}
		s.store.UpdateAIState(state)
		message := fmt.Sprintf("Command permission required for session %s.\nCommand: `%s`\nReason: %s", request.SessionID, request.Command, request.Reason)
		s.emitFn("ai:message", map[string]string{"role": "assistant", "content": message})
		return fmt.Sprintf(`{"status":"approval_required","requestId":%q}`, request.ID), true, nil
	default:
		return `{"error":"unknown policy decision"}`, false, nil
	}
}

func (s *Service) resumePendingNativeToolCall(ctx context.Context, pending *ai.PendingNativeToolCall, toolResult string) error {
	if pending == nil {
		return fmt.Errorf("pending native tool call is required")
	}
	var messages []nativeChatMessage
	if err := json.Unmarshal([]byte(pending.MessagesJSON), &messages); err != nil {
		return fmt.Errorf("restore pending native tool conversation: %w", err)
	}
	messages = append(messages, nativeChatMessage{Role: "tool", ToolCallID: pending.ToolCallID, Content: toolResult})

	state := s.store.AIState()
	provider, err := s.providerByID(state, pending.ProviderID)
	if err != nil {
		return err
	}
	state.PendingNativeToolCall = nil
	s.store.UpdateAIState(state)
	_, _, err = s.runNativeToolLoop(ctx, provider, state.ChatSessionID, state.CommandPolicy, pending.SessionID, pending.UserMessage, messages)
	return err
}

func (s *Service) providerByID(state ai.WorkspaceState, providerID string) (*ai.ProviderDescriptor, error) {
	providerID = strings.TrimSpace(providerID)
	if providerID == "" {
		return nil, fmt.Errorf("AI provider id is required")
	}
	for i := range state.Providers {
		if state.Providers[i].ID != providerID {
			continue
		}
		provider := state.Providers[i]
		if provider.Class == ai.ProviderClassOpenAICompatible {
			token, err := s.store.LoadSecret(securestorage.AIProviderTokenKey(provider.ID))
			if err != nil && !errors.Is(err, securestorage.ErrMasterPasswordRequired) {
				return nil, err
			}
			provider.Token = strings.TrimSpace(token)
		}
		if provider.Class == ai.ProviderClassLocalOpenAI {
			if strings.TrimSpace(provider.Endpoint) == "" {
				provider.Endpoint = localAIEndpoint
			}
			if strings.TrimSpace(provider.LocalPath) == "" || !fileExists(provider.LocalPath) {
				return nil, fmt.Errorf("local AI model is not downloaded")
			}
			if !s.isLocalModelRunning() {
				return nil, fmt.Errorf("local AI model is stopped")
			}
		}
		return &provider, nil
	}
	return nil, fmt.Errorf("AI provider %q not found", providerID)
}

func marshalNativeToolError(format string, args ...any) string {
	return fmt.Sprintf(`{"error":%q}`, fmt.Sprintf(format, args...))
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
