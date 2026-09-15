package app

import (
	"strings"
	"testing"

	"eiksy/internal/domain/ai"
	"eiksy/internal/storage/memory"
)

func TestNativeToolSystemPromptDefinesSafeRemediationLifecycle(t *testing.T) {
	service := NewService(memory.NewStore(), nil, nil)
	prompt := service.nativeToolSystemPrompt(ai.CommandPolicy{}, "session-1")

	for _, step := range []string{"Detect -> Analyze -> Propose -> Approve -> Execute -> Verify", "ssh.diagnostics", "ssh.exec", "Command Policy", "Never claim a remediation succeeded until the execution result and verification support that conclusion"} {
		if !strings.Contains(prompt, step) {
			t.Fatalf("remediation prompt is missing %q: %s", step, prompt)
		}
	}
}
