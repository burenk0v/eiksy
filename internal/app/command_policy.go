package app

import (
	"fmt"
	"strings"

	"eiksy/internal/domain/ai"
)

// commandPolicyDecision is deliberately separate from CommandPermissionMode:
// the former describes what the policy says about a command, while the latter
// describes what a user does with a pending request.
type commandPolicyDecision string

const (
	commandPolicyDecisionAllow commandPolicyDecision = "allow"
	commandPolicyDecisionAsk   commandPolicyDecision = "ask"
	commandPolicyDecisionDeny  commandPolicyDecision = "deny"
)

// defaultCommandRules is intentionally conservative. Command rules are
// authoritative for shell execution: an AllowedTools entry must never bypass
// a deny/ask rule for a specific command.
func defaultCommandRules() []ai.CommandRule {
	return []ai.CommandRule{
		{Pattern: "pwd", Action: ai.CommandPermissionAllow, Description: "Print current directory"},
		{Pattern: "whoami", Action: ai.CommandPermissionAllow, Description: "Print current user"},
		{Pattern: "id", Action: ai.CommandPermissionAllow, Description: "Print user identity"},
		{Pattern: "uname *", Action: ai.CommandPermissionAllow, Description: "Read system information"},
		{Pattern: "hostname", Action: ai.CommandPermissionAllow, Description: "Print hostname"},
		{Pattern: "ls *", Action: ai.CommandPermissionAllow, Description: "List directory contents"},
		{Pattern: "ls", Action: ai.CommandPermissionAllow, Description: "List directory contents"},
		{Pattern: "df -h", Action: ai.CommandPermissionAllow, Description: "Read filesystem usage"},
		{Pattern: "free -h", Action: ai.CommandPermissionAllow, Description: "Read memory usage"},
		{Pattern: "ps *", Action: ai.CommandPermissionAllow, Description: "Read process list"},
		{Pattern: "systemctl status *", Action: ai.CommandPermissionAllow, Description: "Read service status"},
		{Pattern: "systemctl restart *", Action: ai.CommandPermissionAsk, Description: "Restart a service"},
		{Pattern: "systemctl stop *", Action: ai.CommandPermissionAsk, Description: "Stop a service"},
		{Pattern: "systemctl start *", Action: ai.CommandPermissionAsk, Description: "Start a service"},
		{Pattern: "cat *", Action: ai.CommandPermissionAsk, Description: "Read a file; may expose secrets to the AI"},
		{Pattern: "grep *", Action: ai.CommandPermissionAsk, Description: "Search file contents; may expose secrets to the AI"},
		{Pattern: "find *", Action: ai.CommandPermissionAsk, Description: "Search the filesystem"},
		{Pattern: "env", Action: ai.CommandPermissionAsk, Description: "May expose credentials and tokens"},
		{Pattern: "printenv *", Action: ai.CommandPermissionAsk, Description: "May expose credentials and tokens"},
		{Pattern: "history", Action: ai.CommandPermissionAsk, Description: "May expose sensitive command history"},
		{Pattern: "rm *", Action: ai.CommandPermissionDeny, Description: "Destructive file removal"},
		{Pattern: "rmdir *", Action: ai.CommandPermissionDeny, Description: "Destructive directory removal"},
		{Pattern: "shutdown *", Action: ai.CommandPermissionDeny, Description: "System shutdown"},
		{Pattern: "reboot *", Action: ai.CommandPermissionDeny, Description: "System reboot"},
		{Pattern: "poweroff *", Action: ai.CommandPermissionDeny, Description: "System poweroff"},
		{Pattern: "halt *", Action: ai.CommandPermissionDeny, Description: "System halt"},
		{Pattern: "kill *", Action: ai.CommandPermissionDeny, Description: "Process termination"},
		{Pattern: "pkill *", Action: ai.CommandPermissionDeny, Description: "Process termination"},
		{Pattern: "killall *", Action: ai.CommandPermissionDeny, Description: "Process termination"},
		{Pattern: "chmod *", Action: ai.CommandPermissionDeny, Description: "Permission modification"},
		{Pattern: "chown *", Action: ai.CommandPermissionDeny, Description: "Ownership modification"},
		{Pattern: "useradd *", Action: ai.CommandPermissionDeny, Description: "Account creation"},
		{Pattern: "userdel *", Action: ai.CommandPermissionDeny, Description: "Account deletion"},
		{Pattern: "passwd *", Action: ai.CommandPermissionDeny, Description: "Password modification"},
		{Pattern: "mkfs *", Action: ai.CommandPermissionDeny, Description: "Filesystem destruction"},
		{Pattern: "dd *", Action: ai.CommandPermissionDeny, Description: "Raw disk modification"},
	}
}

func normalizeCommandRules(rules []ai.CommandRule) []ai.CommandRule {
	result := make([]ai.CommandRule, 0, len(rules))
	for _, rule := range rules {
		pattern := strings.TrimSpace(rule.Pattern)
		if pattern == "" {
			continue
		}
		action := ai.CommandPermissionAction(strings.ToLower(strings.TrimSpace(string(rule.Action))))
		if action != ai.CommandPermissionAllow && action != ai.CommandPermissionAsk && action != ai.CommandPermissionDeny {
			continue
		}
		result = append(result, ai.CommandRule{
			Pattern: pattern,
			Action: action,
			Description: strings.TrimSpace(rule.Description),
		})
	}
	return result
}

// matchCommandPattern supports an intentionally small glob syntax: '*' means
// any suffix. It does not interpret shell syntax and therefore cannot turn a
// policy pattern into a second shell language.
func matchCommandPattern(pattern, command string) bool {
	pattern = strings.TrimSpace(pattern)
	command = strings.TrimSpace(command)
	if pattern == "" || command == "" {
		return false
	}
	if pattern == command {
		return true
	}
	if !strings.HasSuffix(pattern, "*") {
		return false
	}
	prefix := strings.TrimSpace(strings.TrimSuffix(pattern, "*"))
	if prefix == "" {
		return true
	}
	return strings.HasPrefix(command, prefix)
}

func containsUnsafeShellSyntax(command string) bool {
	for _, token := range []string{";", "&&", "||", "|", ">", "<", "`", "$(", "\n", "\r"} {
		if strings.Contains(command, token) {
			return true
		}
	}
	return false
}

func evaluateCommandPolicy(policy ai.CommandPolicy, toolID, sessionID, command string) (commandPolicyDecision, string) {
	toolID = strings.ToLower(strings.TrimSpace(toolID))
	sessionID = strings.TrimSpace(sessionID)
	command = strings.TrimSpace(command)
	if toolID == "" || command == "" {
		return commandPolicyDecisionDeny, "tool and command are required"
	}
	if !commandToolEnabled(policy, toolID) {
		return commandPolicyDecisionDeny, fmt.Sprintf("tool %q is disabled", toolID)
	}

	// Eiksy sends the command to an interactive shell. Until a real shell AST
	// parser is used, shell composition is denied rather than matched against a
	// simple prefix rule that could be bypassed with `;`, pipes or substitution.
	if toolID == "shell" && containsUnsafeShellSyntax(command) {
		return commandPolicyDecisionDeny, "shell operators and command substitution are not permitted by Command Policy"
	}

	rules := normalizeCommandRules(policy.CommandRules)
	if len(rules) == 0 {
		// Backward-compatible mode for old persisted policies. New normalized
		// policies populate default rules before evaluation.
		if commandAllowedByPolicy(policy, toolID, sessionID) {
			return commandPolicyDecisionAllow, "legacy tool permission"
		}
		return commandPolicyDecisionAsk, "no command rule matched"
	}

	for _, rule := range rules {
		if matchCommandPattern(rule.Pattern, command) {
			switch rule.Action {
			case ai.CommandPermissionDeny:
				return commandPolicyDecisionDeny, rule.Description
			case ai.CommandPermissionAsk:
				return commandPolicyDecisionAsk, rule.Description
			case ai.CommandPermissionAllow:
				return commandPolicyDecisionAllow, rule.Description
			}
		}
	}

	return commandPolicyDecisionAsk, "no command rule matched"
}
