package audit

import (
	"context"
	"time"
)

// Event is the security-relevant execution record persisted by Eiksy.
// Output, credentials and user message content must never be stored here.
type Event struct {
	ID             string    `json:"id"`
	At             time.Time `json:"at"`
	Action         string    `json:"action"`
	ProviderID     string    `json:"providerId,omitempty"`
	ToolID         string    `json:"toolId,omitempty"`
	SessionID      string    `json:"sessionId,omitempty"`
	ChatSessionID  string    `json:"chatSessionId,omitempty"`
	Command        string    `json:"command,omitempty"`
	PolicyDecision string    `json:"policyDecision,omitempty"`
	Approval       string    `json:"approval,omitempty"`
	Result         string    `json:"result,omitempty"`
	ExitCode       int       `json:"exitCode,omitempty"`
	DurationMs     int64     `json:"durationMs,omitempty"`
	ErrorType      string    `json:"errorType,omitempty"`
	Error          string    `json:"error,omitempty"`
}

type Repository interface {
	Append(context.Context, Event) error
	List(context.Context, int) ([]Event, error)
}
