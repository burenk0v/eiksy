package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	"eiksy/internal/domain/ai"
)

// ClearChat removes all messages from the AI chat history.
func (s *Service) ClearChat() {
	state := s.store.AIState()
	state.Messages = []ai.ChatMessage{}
	state.ChatSessionID = fmt.Sprintf("chat-%d", time.Now().UTC().UnixNano())
	s.store.UpdateAIState(state)
}

func (s *Service) UpdateCommandPolicy(policy ai.CommandPolicy) error {
	state := s.store.AIState()
	state.CommandPolicy = normalizeCommandPolicy(policy)
	s.store.UpdateAIState(state)
	return nil
}

func (s *Service) ResolveCommandPolicyRequest(requestID string, mode ai.CommandPermissionMode) error {
	requestID = strings.TrimSpace(requestID)
	if requestID == "" {
		return fmt.Errorf("request id is required")
	}
	state := s.store.AIState()
	policy := normalizeCommandPolicy(state.CommandPolicy)
	index := -1
	var request ai.CommandRequest
	for i, entry := range policy.PendingRequests {
		if entry.ID == requestID {
			index = i
			request = entry
			break
		}
	}
	if index < 0 {
		return fmt.Errorf("command request %q not found", requestID)
	}
	removePending := func() {
		policy.PendingRequests = append(policy.PendingRequests[:index], policy.PendingRequests[index+1:]...)
	}
	if mode == ai.CommandPermissionModeDeny {
		removePending()
		state.CommandPolicy = policy
		if state.PendingNativeToolCall != nil && state.PendingNativeToolCall.RequestID == requestID {
			pending := state.PendingNativeToolCall
			state.PendingNativeToolCall = nil
			s.store.UpdateAIState(state)
			return s.resumePendingNativeToolCall(s.resolveContext(context.Background()), pending, `{"status":"approval_denied","reason":"user denied the command"}`)
		}
		message := fmt.Sprintf("Denied command request for `%s`.", request.Command)
		state.Messages = append(state.Messages, ai.ChatMessage{Role: "assistant", Content: message})
		s.store.UpdateAIState(state)
		s.emitFn("ai:message", map[string]string{"role": "assistant", "content": message})
		return nil
	}
	switch mode {
	case ai.CommandPermissionModeAlways, ai.CommandPermissionModeSession, ai.CommandPermissionModeNow:
	default:
		return fmt.Errorf("unsupported permission mode %q", mode)
	}
	if !commandToolEnabled(policy, request.ToolID) {
		return fmt.Errorf("tool %q is disabled in command policy", request.ToolID)
	}
	decision, reason := evaluateCommandPolicy(policy, request.ToolID, request.SessionID, request.Command)
	if decision == commandPolicyDecisionDeny {
		removePending()
		state.CommandPolicy = policy
		message := fmt.Sprintf("Command denied by Command Policy: `%s`", request.Command)
		if strings.TrimSpace(reason) != "" {
			message += fmt.Sprintf(" (%s)", reason)
		}
		state.Messages = append(state.Messages, ai.ChatMessage{Role: "assistant", Content: message})
		s.store.UpdateAIState(state)
		s.emitFn("ai:message", map[string]string{"role": "assistant", "content": message})
		return nil
	}
	removePending()
	switch mode {
	case ai.CommandPermissionModeAlways:
		policy.CommandRules = append(policy.CommandRules, ai.CommandRule{ToolID: request.ToolID, Pattern: request.Command, Action: ai.CommandPermissionAllow, Description: "Approved by user: always allow this exact command"})
	case ai.CommandPermissionModeSession:
		policy.CommandRules = append(policy.CommandRules, ai.CommandRule{ToolID: request.ToolID, SessionID: request.SessionID, Pattern: request.Command, Action: ai.CommandPermissionAllow, Description: "Approved by user: allow this exact command for this session"})
	}
	policy.CommandRules = normalizeCommandRules(policy.CommandRules)
	state.CommandPolicy = policy
	if state.PendingNativeToolCall != nil && state.PendingNativeToolCall.RequestID == requestID {
		pending := state.PendingNativeToolCall
		state.PendingNativeToolCall = nil
		s.store.UpdateAIState(state)
		output, err := s.executeSessionCommandWithOutput(request.SessionID, request.Command)
		if err != nil {
			return s.resumePendingNativeToolCall(s.resolveContext(context.Background()), pending, fmt.Sprintf(`{"status":"execution_failed","message":%q,"output":%q}`, err.Error(), output))
		}
		return s.resumePendingNativeToolCall(s.resolveContext(context.Background()), pending, fmt.Sprintf(`{"status":"executed","sessionId":%q,"command":%q,"output":%q}`, request.SessionID, request.Command, output))
	}
	if request.ToolID != "shell" {
		return fmt.Errorf("tool %q does not support command dispatch", request.ToolID)
	}
	message := ""
	if err := s.executeSessionCommand(request.SessionID, request.Command); err != nil {
		message = fmt.Sprintf("Command execution failed: %v", err)
	} else {
		message = fmt.Sprintf("Approved and executed command in session %s: `%s`", request.SessionID, request.Command)
	}
	state.Messages = append(state.Messages, ai.ChatMessage{Role: "assistant", Content: message})
	s.store.UpdateAIState(state)
	s.emitFn("ai:message", map[string]string{"role": "assistant", "content": message})
	return nil
}
func normalizeCommandPolicy(policy ai.CommandPolicy) ai.CommandPolicy {
	defaults := []ai.CommandTool{
		{ID: "shell", Name: "Shell command", Description: "Run command in active SSH session", Enabled: true},
		{ID: "sftp", Name: "SFTP operations", Description: "Browse and edit files over SFTP", Enabled: true},
		{ID: "search", Name: "Search", Description: "Run grep/find-like queries on host", Enabled: true},
	}
	toolByID := make(map[string]ai.CommandTool, len(defaults))
	order := make([]string, 0, len(defaults))
	for _, tool := range defaults {
		toolByID[tool.ID] = tool
		order = append(order, tool.ID)
	}
	for _, tool := range policy.Tools {
		id := strings.ToLower(strings.TrimSpace(tool.ID))
		if id == "" {
			continue
		}
		clean := ai.CommandTool{
			ID:          id,
			Name:        strings.TrimSpace(tool.Name),
			Description: strings.TrimSpace(tool.Description),
			Enabled:     tool.Enabled,
		}
		if clean.Name == "" {
			clean.Name = id
		}
		if _, exists := toolByID[id]; !exists {
			order = append(order, id)
		}
		toolByID[id] = clean
	}
	normalizedTools := make([]ai.CommandTool, 0, len(order))
	for _, id := range order {
		normalizedTools = append(normalizedTools, toolByID[id])
	}
	policy.Tools = normalizedTools
	policy.LocalDocsPath = strings.TrimSpace(policy.LocalDocsPath)
	policy.AllowedTools = uniqueStrings(policy.AllowedTools)
	if policy.SessionAllowedTools == nil {
		policy.SessionAllowedTools = map[string][]string{}
	}
	for sessionID, tools := range policy.SessionAllowedTools {
		trimmedSession := strings.TrimSpace(sessionID)
		if trimmedSession == "" {
			delete(policy.SessionAllowedTools, sessionID)
			continue
		}
		policy.SessionAllowedTools[trimmedSession] = uniqueStrings(tools)
		if trimmedSession != sessionID {
			delete(policy.SessionAllowedTools, sessionID)
		}
	}
	if policy.PendingRequests == nil {
		policy.PendingRequests = []ai.CommandRequest{}
	}
	filteredRequests := make([]ai.CommandRequest, 0, len(policy.PendingRequests))
	for _, request := range policy.PendingRequests {
		command := strings.TrimSpace(request.Command)
		toolID := strings.ToLower(strings.TrimSpace(request.ToolID))
		if command == "" || toolID == "" {
			continue
		}
		filteredRequests = append(filteredRequests, ai.CommandRequest{
			ID:          strings.TrimSpace(request.ID),
			ToolID:      toolID,
			SessionID:   strings.TrimSpace(request.SessionID),
			Command:     command,
			Reason:      strings.TrimSpace(request.Reason),
			RequestedAt: strings.TrimSpace(request.RequestedAt),
		})
	}
	policy.PendingRequests = filteredRequests
	return policy
}

func uniqueStrings(values []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		normalized := strings.ToLower(strings.TrimSpace(value))
		if normalized == "" {
			continue
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}
	return result
}

func commandAllowedByPolicy(policy ai.CommandPolicy, toolID, sessionID string) bool {
	normalized := normalizeCommandPolicy(policy)
	toolID = strings.ToLower(strings.TrimSpace(toolID))
	sessionID = strings.TrimSpace(sessionID)
	if toolID == "" {
		return false
	}
	if !commandToolEnabled(normalized, toolID) {
		return false
	}
	for _, allowed := range normalized.AllowedTools {
		if allowed == toolID {
			return true
		}
	}
	for _, allowed := range normalized.SessionAllowedTools[sessionID] {
		if allowed == toolID {
			return true
		}
	}
	return false
}

func commandToolEnabled(policy ai.CommandPolicy, toolID string) bool {
	toolID = strings.ToLower(strings.TrimSpace(toolID))
	if toolID == "" {
		return false
	}
	normalized := normalizeCommandPolicy(policy)
	for _, tool := range normalized.Tools {
		if tool.ID == toolID {
			return tool.Enabled
		}
	}
	return false
}
