package app

import (
	"fmt"
	"strings"

	"eiksy/internal/domain/ai"
)

// ListChatSessions returns persistent conversation metadata without message content.
// Chat messages remain owned by the secure application storage layer.
func (s *Service) ListChatSessions() []ai.ChatSession {
	if s == nil || s.store == nil { return []ai.ChatSession{} }
	return s.store.ListChatSessions()
}

// CreateChatSession creates and activates a persistent AI conversation.
func (s *Service) CreateChatSession(title string) (ai.ChatSession, error) {
	if s == nil || s.store == nil { return ai.ChatSession{}, fmt.Errorf("application service is unavailable") }
	return s.store.CreateChatSession(strings.TrimSpace(title))
}

// SelectChatSession activates an existing persistent AI conversation.
func (s *Service) SelectChatSession(id string) error {
	if s == nil || s.store == nil { return fmt.Errorf("application service is unavailable") }
	return s.store.SelectChatSession(strings.TrimSpace(id))
}


// ForkChatSession creates a new persistent conversation with a copy of the source session's messages.
func (s *Service) ForkChatSession(id, title string) (ai.ChatSession, error) {
	if s == nil || s.store == nil {
		return ai.ChatSession{}, fmt.Errorf("application service is unavailable")
	}
	return s.store.ForkChatSession(strings.TrimSpace(id), strings.TrimSpace(title))
}
