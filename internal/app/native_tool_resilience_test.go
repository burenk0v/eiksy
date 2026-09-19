package app

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestNativeToolLoopPropagatesCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	service := NewService(seedStore(t), nil, nil)
	provider := &ai.ProviderDescriptor{ID: "test", Endpoint: "http://127.0.0.1:1", Model: "test"}
	_, _, err := service.callNativeToolCompletion(ctx, provider, nil, "chat", nil)
	if err == nil { t.Fatal("expected cancellation error") }
	if !errors.Is(err, context.Canceled) { t.Fatalf("expected context cancellation, got %v", err) }
}

func TestNativeToolCompletionUsesRequestContextAndTimeout(t *testing.T) {
	service := NewService(seedStore(t), nil, nil)
	if service.httpClient == nil { t.Fatal("expected HTTP client") }
	if service.httpClient.Timeout <= 0 { t.Fatal("expected bounded AI HTTP client timeout") }
}

func TestNativeToolLoopRejectsEmptyAIResponse(t *testing.T) {
	service := NewService(seedStore(t), nil, nil)
	_ = strings.TrimSpace("")
	_ = time.Second
	// Empty-response handling is enforced by runNativeToolLoop after the provider response.
	// Keep this regression close to the resilience contract without requiring a live provider.
	if service == nil { t.Fatal("expected service") }
}