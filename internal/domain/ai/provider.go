package ai

import "context"

// Provider is the runtime boundary for an AI backend. Credentials and
// transport-specific configuration stay inside the provider implementation.
type Provider interface {
	ID() string
	Chat(context.Context, ChatRequest) (ChatResponse, error)
}

type ChatRequest struct {
	Model    string
	Messages []ChatMessage
}

type ChatResponse struct {
	Message ChatMessage
}
