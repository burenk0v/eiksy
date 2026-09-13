from pathlib import Path

SERVICE = Path('internal/app/service.go')
TEST = Path('internal/app/command_policy_test.go')
SERVICE_TEST = Path('internal/app/service_test.go')
SCRIPT = Path('scripts/command_policy_v2_finalize.py')
WORKFLOW = Path('.github/workflows/command-policy-v2-finalize.yml')

NEW_RESOLVE = r'''func (s *Service) ResolveCommandPolicyRequest(requestID string, mode ai.CommandPermissionMode) error {
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
	if mode == ai.CommandPermissionModeDeny {
		policy.PendingRequests = append(policy.PendingRequests[:index], policy.PendingRequests[index+1:]...)
		state.CommandPolicy = policy
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
		policy.PendingRequests = append(policy.PendingRequests[:index], policy.PendingRequests[index+1:]...)
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

	policy.PendingRequests = append(policy.PendingRequests[:index], policy.PendingRequests[index+1:]...)
	switch mode {
	case ai.CommandPermissionModeAlways:
		policy.CommandRules = append(policy.CommandRules, ai.CommandRule{
			ToolID: request.ToolID, Pattern: request.Command, Action: ai.CommandPermissionAllow,
			Description: "Approved by user: always allow this exact command",
		})
	case ai.CommandPermissionModeSession:
		policy.CommandRules = append(policy.CommandRules, ai.CommandRule{
			ToolID: request.ToolID, SessionID: request.SessionID, Pattern: request.Command, Action: ai.CommandPermissionAllow,
			Description: "Approved by user: allow this exact command for this session",
		})
	}
	policy.CommandRules = normalizeCommandRules(policy.CommandRules)
	state.CommandPolicy = policy

	message := ""
	if request.ToolID != "shell" {
		message = fmt.Sprintf("Approved tool %s request. Command dispatch is currently supported only for shell, so `%s` was not executed.", request.ToolID, request.Command)
	} else if err := s.executeSessionCommand(request.SessionID, request.Command); err != nil {
		message = fmt.Sprintf("Command execution failed: %v", err)
	} else {
		message = fmt.Sprintf("Approved and executed command in session %s: `%s`", request.SessionID, request.Command)
	}
	state.Messages = append(state.Messages, ai.ChatMessage{Role: "assistant", Content: message})
	s.store.UpdateAIState(state)
	s.emitFn("ai:message", map[string]string{"role": "assistant", "content": message})
	return nil
}

'''

OLD_MARKER = 'func (s *Service) ResolveCommandPolicyRequest('
END_MARKER = '// AcceptSSHHostKey trusts the pending unknown host key'


def patch_service():
    text = SERVICE.read_text()
    start = text.find(OLD_MARKER)
    end = text.find(END_MARKER, start)
    if start < 0 or end < 0:
        raise SystemExit('ResolveCommandPolicyRequest block not found')
    text = text[:start] + NEW_RESOLVE + text[end:]
    old = '''} else if allowed := commandAllowedByPolicy(state.CommandPolicy, commandRequest.ToolID, commandRequest.SessionID); allowed {
			if err := s.executeSessionCommand(commandRequest.SessionID, commandRequest.Command); err != nil {
				reply = fmt.Sprintf("Command execution failed: %v", err)
			} else {
				reply = fmt.Sprintf("Executed command in session %s via tool %s:\\n`%s`", commandRequest.SessionID, commandRequest.ToolID, commandRequest.Command)
			}
		} else {'''
    new = '''} else if decision, reason := evaluateCommandPolicy(state.CommandPolicy, commandRequest.ToolID, commandRequest.SessionID, commandRequest.Command); decision == commandPolicyDecisionAllow {
			if err := s.executeSessionCommand(commandRequest.SessionID, commandRequest.Command); err != nil {
				reply = fmt.Sprintf("Command execution failed: %v", err)
			} else {
				reply = fmt.Sprintf("Executed command in session %s via tool %s:\\n`%s`", commandRequest.SessionID, commandRequest.ToolID, commandRequest.Command)
			}
		} else if decision == commandPolicyDecisionDeny {
			reply = fmt.Sprintf("Command denied by Command Policy: `%s`", commandRequest.Command)
			if strings.TrimSpace(reason) != "" {
				reply += fmt.Sprintf(" (%s)", reason)
			}
		} else {'''
    if old not in text:
        raise SystemExit('command dispatch block not found')
    text = text.replace(old, new, 1)
    SERVICE.write_text(text)


def patch_policy_tests():
    text = TEST.read_text()
    additions = r'''

func TestEvaluateCommandPolicyScopesAndPrecedence(t *testing.T) {
	policy := ai.CommandPolicy{
		Tools: []ai.CommandTool{{ID: "shell", Enabled: true}, {ID: "other", Enabled: true}},
		CommandRules: []ai.CommandRule{
			{Pattern: "cat *", Action: ai.CommandPermissionAsk},
			{ToolID: "shell", Pattern: "cat /etc/hosts", Action: ai.CommandPermissionAllow},
			{ToolID: "shell", SessionID: "session-2", Pattern: "cat /etc/hosts", Action: ai.CommandPermissionDeny},
			{ToolID: "other", Pattern: "cat /etc/hosts", Action: ai.CommandPermissionAllow},
		},
	}

	if got, _ := evaluateCommandPolicy(policy, "shell", "session-1", "cat /etc/hosts"); got != commandPolicyDecisionAllow {
		t.Fatalf("exact approval = %q, want allow", got)
	}
	if got, _ := evaluateCommandPolicy(policy, "shell", "session-2", "cat /etc/hosts"); got != commandPolicyDecisionDeny {
		t.Fatalf("explicit session deny = %q, want deny", got)
	}
	if got, _ := evaluateCommandPolicy(policy, "shell", "session-1", "cat /etc/passwd"); got != commandPolicyDecisionAsk {
		t.Fatalf("broad ask = %q, want ask", got)
	}
	if got, _ := evaluateCommandPolicy(policy, "other", "session-1", "cat /etc/hosts"); got != commandPolicyDecisionAllow {
		t.Fatalf("tool-scoped rule = %q, want allow", got)
	}
}

func TestEvaluateCommandPolicyRejectsSecretExpansion(t *testing.T) {
	policy := ai.CommandPolicy{
		Tools: []ai.CommandTool{{ID: "shell", Enabled: true}},
		CommandRules: []ai.CommandRule{{Pattern: "echo *", Action: ai.CommandPermissionAllow}},
	}
	for _, command := range []string{"echo $TOKEN", "echo ${TOKEN}", "echo foo\\bar"} {
		if got, _ := evaluateCommandPolicy(policy, "shell", "session-1", command); got != commandPolicyDecisionDeny {
			t.Errorf("secret/escape command %q = %q, want deny", command, got)
		}
	}
}
'''
    if 'TestEvaluateCommandPolicyScopesAndPrecedence' not in text:
        TEST.write_text(text + additions)


def patch_service_tests():
    text = SERVICE_TEST.read_text()
    old1 = '''	if !slices.Contains(updated.CommandPolicy.SessionAllowedTools[tab.ID], "shell") {
		t.Fatalf("expected shell to be granted for session %s", tab.ID)
	}
'''
    new1 = '''	if !hasCommandRule(updated.CommandPolicy.CommandRules, "shell", tab.ID, "uname -a", ai.CommandPermissionAllow) {
		t.Fatalf("expected exact shell command rule for session %s", tab.ID)
	}
'''
    if old1 not in text:
        raise SystemExit('session approval expectation not found')
    text = text.replace(old1, new1, 1)

    old2 = '''	if !slices.Contains(updated.CommandPolicy.AllowedTools, "shell") {
		t.Fatalf("expected global shell grant to persist, got %v", updated.CommandPolicy.AllowedTools)
	}
'''
    new2 = '''	if !hasCommandRule(updated.CommandPolicy.CommandRules, "shell", "", "hostname", ai.CommandPermissionAllow) {
		t.Fatalf("expected exact global shell command rule to persist, got %v", updated.CommandPolicy.CommandRules)
	}
'''
    if old2 not in text:
        raise SystemExit('always approval expectation not found')
    text = text.replace(old2, new2, 1)

    helper = r'''

func hasCommandRule(rules []ai.CommandRule, toolID, sessionID, pattern string, action ai.CommandPermissionAction) bool {
	for _, rule := range rules {
		if rule.ToolID == toolID && rule.SessionID == sessionID && rule.Pattern == pattern && rule.Action == action {
			return true
		}
	}
	return false
}
'''
    marker = '\nfunc TestBuildPortForwardSpecsWithRemoteTarget'
    if 'func hasCommandRule(' not in text:
        pos = text.find(marker)
        if pos < 0:
            raise SystemExit('service test insertion marker not found')
        text = text[:pos] + helper + text[pos:]
    SERVICE_TEST.write_text(text)


def main():
    patch_service()
    patch_policy_tests()
    patch_service_tests()
    if SCRIPT.exists():
        SCRIPT.unlink()
    if WORKFLOW.exists():
        WORKFLOW.unlink()


if __name__ == '__main__':
    main()
