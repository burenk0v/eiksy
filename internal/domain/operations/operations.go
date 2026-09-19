package operations

import (
	"time"

	"eiksy/internal/domain/sessions"
)

type Kind string

const (
	KindCommand Kind = "command"
	KindFileWrite Kind = "file_write"
	KindDiagnostic Kind = "diagnostic"
)

type Status string

const (
	StatusPending Status = "pending"
	StatusApprovalRequired Status = "approval_required"
	StatusRunning Status = "running"
	StatusSucceeded Status = "succeeded"
	StatusFailed Status = "failed"
	StatusDenied Status = "denied"
)

type Operation struct {
	ID               string                         `json:"id"`
	Kind             Kind                           `json:"kind"`
	SessionID        string                         `json:"sessionId"`
	Target           string                         `json:"target"`
	Input            string                         `json:"input"`
	Status           Status                         `json:"status"`
	ApprovalRequired bool                           `json:"approvalRequired"`
	CreatedAt        time.Time                      `json:"createdAt"`
	StartedAt        *time.Time                     `json:"startedAt,omitempty"`
	FinishedAt       *time.Time                     `json:"finishedAt,omitempty"`
	Result           *sessions.CommandExecutionResult `json:"result,omitempty"`
	Error            string                         `json:"error,omitempty"`
}

func NewCommand(id, sessionID, command string) Operation {
	return Operation{
		ID: id,
		Kind: KindCommand,
		SessionID: sessionID,
		Target: sessionID,
		Input: command,
		Status: StatusPending,
		CreatedAt: time.Now().UTC(),
	}
}

func (o *Operation) Start(now time.Time) {
	now = now.UTC()
	o.Status = StatusRunning
	o.StartedAt = &now
}

func (o *Operation) Complete(result sessions.CommandExecutionResult, now time.Time) {
	now = now.UTC()
	o.Result = &result
	o.FinishedAt = &now
	if result.Success {
		o.Status = StatusSucceeded
	} else {
		o.Status = StatusFailed
	}
	if result.Error != "" {
		o.Error = result.Error
	}
}

func (o *Operation) Deny(now time.Time, reason string) {
	now = now.UTC()
	o.Status = StatusDenied
	o.FinishedAt = &now
	o.Error = reason
}
