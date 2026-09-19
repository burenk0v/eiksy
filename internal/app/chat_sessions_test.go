package app

import (
	"testing"

	"eiksy/internal/domain/ai"
	"eiksy/internal/storage/memory"
)

func TestChatSessionsCreateAndSelect(t *testing.T) {
	store := memory.NewStore()
	service := NewService(store, nil, nil)

	sessions := service.ListChatSessions()
	if len(sessions) != 1 { t.Fatalf("expected one default chat session, got %d", len(sessions)) }
	firstID := sessions[0].ID

	state := store.AIState()
	state.Messages = append(state.Messages, ai.ChatMessage{Role: "user", Content: "remember this"})
	if err := store.UpdateAIState(state); err != nil { t.Fatalf("update state: %v", err) }

	created, err := service.CreateChatSession("Debug production")
	if err != nil { t.Fatalf("create session: %v", err) }
	if created.Title != "Debug production" { t.Fatalf("unexpected title %q", created.Title) }
	if created.ID == firstID { t.Fatal("new session reused existing id") }
	if len(store.AIState().Messages) != 0 { t.Fatal("new session should start with empty messages") }

	if err := service.SelectChatSession(firstID); err != nil { t.Fatalf("select session: %v", err) }
	state = store.AIState()
	if state.ChatSessionID != firstID { t.Fatalf("expected active session %q, got %q", firstID, state.ChatSessionID) }
	if len(state.Messages) != 2 { t.Fatalf("expected restored messages, got %d", len(state.Messages)) }
	if state.Messages[1].Content != "remember this" { t.Fatalf("unexpected restored message %q", state.Messages[1].Content) }
}

func TestSelectChatSessionRejectsUnknownID(t *testing.T) {
	service := NewService(memory.NewStore(), nil, nil)
	if err := service.SelectChatSession("missing"); err == nil { t.Fatal("expected unknown session error") }
}
