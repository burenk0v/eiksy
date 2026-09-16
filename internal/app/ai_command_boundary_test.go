package app

import (
	"strings"
	"testing"

	"eiksy/internal/domain/sessions"
)

func TestBoundAICommandResultTruncatesAllUnboundedFields(t *testing.T) {
	large := strings.Repeat("x", maxAICommandOutput+100)
	result := boundAICommandResult(sessions.CommandExecutionResult{
		Stdout: large,
		Stderr: large,
		Error:  large,
	})

	for name, value := range map[string]string{
		"stdout": result.Stdout,
		"stderr": result.Stderr,
		"error":  result.Error,
	} {
		if len(value) > maxAICommandOutput+len("\n[output truncated by Eiksy]") {
			t.Fatalf("%s exceeds bounded size: %d", name, len(value))
		}
		if !strings.HasSuffix(value, "[output truncated by Eiksy]") {
			t.Fatalf("%s was not truncated", name)
		}
	}
}

func TestExecuteSessionCommandResultRejectsOversizedCommand(t *testing.T) {
	service := &Service{}
	_, err := service.executeSessionCommandResult("session-1", strings.Repeat("x", maxAICommandLength+1))
	if err == nil {
		t.Fatal("expected oversized command to be rejected")
	}
	if !strings.Contains(err.Error(), "maximum length") {
		t.Fatalf("unexpected error: %v", err)
	}
}
