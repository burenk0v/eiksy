package main

import "testing"

func TestPrepareAIChatMessageWithoutSessionContext(t *testing.T) {
	message, sessionID := prepareAIChatMessage("Explain this error", "session-1")
	if message != "Explain this error" {
		t.Fatalf("unexpected message: %q", message)
	}
	if sessionID != "" {
		t.Fatalf("expected no session context, got %q", sessionID)
	}
}

func TestPrepareAIChatMessageWithSessionContext(t *testing.T) {
	message, sessionID := prepareAIChatMessage("Explain this error"+aiSessionContextMarker+"legacy output", "session-1")
	if message != "Explain this error" {
		t.Fatalf("expected console attachment to become an opt-in marker only, got %q", message)
	}
	if sessionID != "session-1" {
		t.Fatalf("expected opted-in session context, got %q", sessionID)
	}
}

func TestPrepareAIChatMessageDoesNotOptInWithoutActiveSession(t *testing.T) {
	message, sessionID := prepareAIChatMessage("Explain this error"+aiSessionContextMarker+"output", "")
	if message != "Explain this error" {
		t.Fatalf("unexpected message: %q", message)
	}
	if sessionID != "" {
		t.Fatalf("expected no session context without active session, got %q", sessionID)
	}
}
