package operations

import "strings"

import (
	"strings"
	"testing"
	"time"

	"eiksy/internal/domain/sessions"
)

func TestCommandOperationLifecycle(t *testing.T) {
	created := time.Date(2026, 9, 19, 15, 0, 0, 0, time.UTC)
	started := created.Add(time.Second)
	finished := started.Add(250 * time.Millisecond)

	op := NewCommand("op-1", "session-1", "uname -a")
	if op.ID != "op-1" || op.Kind != KindCommand || op.SessionID != "session-1" || op.Target != "session-1" {
		t.Fatalf("unexpected command operation: %+v", op)
	}
	if op.Status != StatusPending || op.CreatedAt.IsZero() {
		t.Fatalf("expected pending operation with creation time: %+v", op)
	}

	op.Start(started)
	if op.Status != StatusRunning || op.StartedAt == nil || !op.StartedAt.Equal(started) {
		t.Fatalf("expected running operation: %+v", op)
	}

	result := sessions.CommandExecutionResult{Success: true, ExitCode: 0, Stdout: "ok"}
	op.Complete(result, finished)
	if op.Status != StatusSucceeded || op.FinishedAt == nil || !op.FinishedAt.Equal(finished) {
		t.Fatalf("expected succeeded operation: %+v", op)
	}
	if op.Result == nil || op.Result.Stdout != "ok" {
		t.Fatalf("expected operation result: %+v", op.Result)
	}
}

func TestOperationFailedAndDeniedStates(t *testing.T) {
	op := NewCommand("op-2", "session-1", "bad-command")
	now := time.Now().UTC()

	failure := sessions.CommandExecutionResult{
		Success: false,
		ExitCode: 127,
		ErrorType: sessions.CommandExecutionErrorNonZero,
		Error: "command failed",
	}
	op.Start(now)
	op.Complete(failure, now.Add(time.Second))
	if op.Status != StatusFailed || op.Error != "command failed" {
		t.Fatalf("expected failed operation: %+v", op)
	}

	denied := NewCommand("op-3", "session-1", "rm -rf /")
	denied.Deny(now, "requires user approval")
	if denied.Status != StatusDenied || denied.Error != "requires user approval" || denied.FinishedAt == nil {
		t.Fatalf("expected denied operation: %+v", denied)
	}

	if !strings.Contains(string(denied.Status), "denied") {
		t.Fatalf("unexpected denied status: %q", denied.Status)
	}

func TestNewResultBoundsDownstreamOutput(t *testing.T) {
	long := strings.Repeat("x", maxResultOutput+100)
	result := NewResult(sessions.CommandExecutionResult{
		Success: true,
		ExitCode: 0,
		DurationMs: 42,
		Stdout: long,
		Stderr: long,
		Error: long,
	})
	if len(result.Stdout) != maxResultOutput+len("\n[output truncated]") {
		t.Fatalf("expected bounded stdout, got %d bytes", len(result.Stdout))
	}
	if len(result.Stderr) != maxResultOutput+len("\n[output truncated]") {
		t.Fatalf("expected bounded stderr, got %d bytes", len(result.Stderr))
	}
	if len(result.Error) != maxResultOutput+len("\n[output truncated]") {
		t.Fatalf("expected bounded error, got %d bytes", len(result.Error))
	}
	if result.ExitCode != 0 || result.DurationMs != 42 || !result.Success {
		t.Fatalf("expected execution metadata to be preserved, got %+v", result)
	}
}

func TestOperationCompleteStoresBoundedResult(t *testing.T) {
	op := NewCommand("op-1", "session-1", "uname -a")
	now := time.Date(2026, 9, 19, 12, 0, 0, 0, time.UTC)
	long := strings.Repeat("y", maxResultOutput+50)
	op.Start(now)
	op.Complete(sessions.CommandExecutionResult{Success: false, ExitCode: 2, Stdout: long, Error: long}, now.Add(time.Second))
	if op.Status != StatusFailed {
		t.Fatalf("expected failed status, got %q", op.Status)
	}
	if op.Result == nil {
		t.Fatal("expected operation result")
	}
	if len(op.Result.Stdout) > maxResultOutput+len("\n[output truncated]") {
		t.Fatalf("operation result stdout is unbounded: %d", len(op.Result.Stdout))
	}
	if len(op.Result.Error) > maxResultOutput+len("\n[output truncated]") {
		t.Fatalf("operation result error is unbounded: %d", len(op.Result.Error))
	}
}

}
