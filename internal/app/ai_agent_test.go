package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	agentai "eiksy/internal/ai"
	domainai "eiksy/internal/domain/ai"
	"eiksy/internal/storage/memory"
)

func TestAIBackendAgentRunUsesExistingAIService(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"backend reply"}}]}`))
	}))
	defer server.Close()

	store := memory.NewStore()
	state := store.AIState()
	state.Messages = nil
	state.Providers = []domainai.ProviderDescriptor{{
		ID:         "provider-1",
		Class:      domainai.ProviderClassOpenAICompatible,
		Model:      "test-model",
		Endpoint:   server.URL + "/v1",
		Selected:   true,
		Configured: true,
	}}
	if err := store.UpdateAIState(state); err != nil {
		t.Fatalf("seed AI state: %v", err)
	}

	service := NewService(store, nil, nil)
	agent := NewAIBackendAgent(service)

	events, err := agent.Run(context.Background(), "session-1", "hello")
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	var got []agentai.Event
	for event := range events {
		got = append(got, event)
	}

	if len(got) != 3 {
		t.Fatalf("expected 3 events, got %d: %#v", len(got), got)
	}
	if got[0].Type != agentai.EventMessageStarted {
		t.Fatalf("expected message started, got %q", got[0].Type)
	}
	if got[1].Type != agentai.EventTextDelta || got[1].Content != "backend reply" {
		t.Fatalf("unexpected text event: %#v", got[1])
	}
	if got[2].Type != agentai.EventMessageFinished || got[2].Content != "backend reply" {
		t.Fatalf("unexpected completion event: %#v", got[2])
	}

	finalState := store.AIState()
	if len(finalState.Messages) != 2 ||
		finalState.Messages[0].Role != "user" ||
		finalState.Messages[0].Content != "hello" ||
		finalState.Messages[1].Role != "assistant" ||
		finalState.Messages[1].Content != "backend reply" {
		t.Fatalf("unexpected persisted messages: %#v", finalState.Messages)
	}
}

func TestAIBackendAgentRunReportsConfigurationError(t *testing.T) {
	store := memory.NewStore()
	state := store.AIState()
	state.Messages = nil
	state.Providers = nil
	if err := store.UpdateAIState(state); err != nil {
		t.Fatalf("seed AI state: %v", err)
	}

	agent := NewAIBackendAgent(NewService(store, nil, nil))
	events, err := agent.Run(context.Background(), "session-1", "hello")
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	var got []agentai.Event
	for event := range events {
		got = append(got, event)
	}
	if len(got) != 2 {
		t.Fatalf("expected message started and error, got %#v", got)
	}
	if got[0].Type != agentai.EventMessageStarted {
		t.Fatalf("expected message started, got %q", got[0].Type)
	}
	if got[1].Type != agentai.EventError || got[1].Err == nil {
		t.Fatalf("expected error event, got %#v", got[1])
	}
}

func TestAIBackendAgentRunReportsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	agent := NewAIBackendAgent(NewService(memory.NewStore(), nil, nil))
	events, err := agent.Run(ctx, "session-1", "hello")
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	event, ok := <-events
	if !ok {
		t.Fatal("expected cancellation event")
	}
	if event.Type != agentai.EventCancellation {
		t.Fatalf("expected cancellation, got %q", event.Type)
	}
}
