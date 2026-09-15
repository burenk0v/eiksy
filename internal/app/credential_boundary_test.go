package app

import (
	"testing"

	"eiksy/internal/domain/ai"
)

func TestDefaultCommandPolicyDeniesDirectSecretMaterial(t *testing.T) {
	policy := ai.CommandPolicy{
		Tools:        []ai.CommandTool{{ID: "shell", Enabled: true}},
		CommandRules: defaultCommandRules(),
	}

	for _, command := range []string{
		"cat /etc/shadow",
		"cat /etc/gshadow",
		"cat ~/.ssh/id_rsa",
		"cat .env",
		"cat .env.production",
		"env",
		"printenv HOME",
		"history",
	} {
		got, reason := evaluateCommandPolicy(policy, "shell", "session-1", command)
		if got != commandPolicyDecisionDeny {
			t.Errorf("secret command %q = %q (%s), want deny", command, got, reason)
		}
	}
}
