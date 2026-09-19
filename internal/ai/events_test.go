package ai

import (
	"context"
	"testing"
)

func TestEventTypes(t *testing.T) {
	want := []EventType{
		EventMessageStarted,
		EventTextDelta,
		EventToolStarted,
		EventToolOutput,
		EventToolFinished,
		EventMessageFinished,
		EventError,
		EventCancellation,
	}
	seen := map[EventType]bool{}
	for _, typ := range want {
		if typ == "" {
			t.Fatal("event type must not be empty")
		}
		if seen[typ] {
			t.Fatalf("duplicate event type %q", typ)
		}
		seen[typ] = true
	}
}

func TestAgentContractCanBeImplemented(t *testing.T) {
	var agent Agent = fakeAgent{}
	events, err := agent.Run(context.Background(), "session-1", "hello")
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	event, ok := <-events
	if !ok {
		t.Fatal("expected one event")
	}
	if event.Type != EventMessageFinished || event.Content != "hello" {
		t.Fatalf("unexpected event: %+v", event)
	}
}

type fakeAgent struct{}

func (fakeAgent) Run(_ context.Context, _ string, input string) (<-chan Event, error) {
	ch := make(chan Event, 1)
	ch <- Event{Type: EventMessageFinished, Content: input}
	close(ch)
	return ch, nil
}
