package ai

import (
	"context"
	"testing"
)

type fakeProvider struct{}

func (fakeProvider) ID() string { return "fake" }
func (fakeProvider) Chat(context.Context, ChatRequest) (ChatResponse, error) {
	return ChatResponse{Message: ChatMessage{Role: "assistant", Content: "ok"}}, nil
}

func TestProviderContract(t *testing.T) {
	var provider Provider = fakeProvider{}
	response, err := provider.Chat(context.Background(), ChatRequest{
		Model:    "test-model",
		Messages: []ChatMessage{{Role: "user", Content: "hello"}},
	})
	if err != nil {
		t.Fatalf("chat: %v", err)
	}
	if provider.ID() != "fake" {
		t.Fatalf("unexpected provider id: %q", provider.ID())
	}
	if response.Message.Content != "ok" {
		t.Fatalf("unexpected response: %#v", response.Message)
	}
}
