package app

import (
	"errors"
	"strings"
	"testing"

	"eiksy/internal/domain/ai"
	"eiksy/internal/storage/memory"
)

func TestRecordCommandPolicyResolutionApproved(t *testing.T) {
	service := NewService(memory.NewStore(), nil, nil)
	request := ai.CommandRequest{ID: "cmdreq-1", ToolID: "shell", SessionID: "ssh-prod", Command: "systemctl restart nginx"}

	service.RecordCommandPolicyResolutionForApp(request, ai.CommandPermissionModeNow, nil)
	trail := service.GetCommandAuditTrail()
	if len(trail) != 1 {
		t.Fatalf("expected one audit event, got %d", len(trail))
	}
	event := trail[0]
	if event.Approval != "approved" || event.Result != "approved" {
		t.Fatalf("unexpected approval lifecycle event: %+v", event)
	}
	if event.SessionID != request.SessionID || event.Command != request.Command {
		t.Fatalf("expected request context to be preserved: %+v", event)
	}
}

func TestRecordCommandPolicyResolutionDenied(t *testing.T) {
	service := NewService(memory.NewStore(), nil, nil)
	request := ai.CommandRequest{ID: "cmdreq-2", ToolID: "shell", SessionID: "ssh-prod", Command: "systemctl restart nginx"}

	service.RecordCommandPolicyResolutionForApp(request, ai.CommandPermissionModeDeny, nil)
	trail := service.GetCommandAuditTrail()
	if len(trail) != 1 {
		t.Fatalf("expected one audit event, got %d", len(trail))
	}
	if trail[0].Approval != "denied" || trail[0].Result != "approval_denied" || trail[0].ErrorType != "approval_denied" {
		t.Fatalf("unexpected denial lifecycle event: %+v", trail[0])
	}
}

func TestRecordCommandPolicyResolutionDoesNotLeakResolutionErrorSecrets(t *testing.T) {
	service := NewService(memory.NewStore(), nil, nil)
	request := ai.CommandRequest{ID: "cmdreq-3", ToolID: "shell", SessionID: "ssh-prod", Command: "cat /etc/app.conf"}
	secretError := errors.New("ssh failed with token=super-secret")

	service.RecordCommandPolicyResolutionForApp(request, ai.CommandPermissionModeNow, secretError)
	trail := service.GetCommandAuditTrail()
	if len(trail) != 1 {
		t.Fatalf("expected one audit event, got %d", len(trail))
	}
	event := trail[0]
	if event.Result != "approval_resolution_failed" {
		t.Fatalf("expected resolution failure, got %+v", event)
	}
	if strings.Contains(event.Error, "super-secret") {
		t.Fatalf("audit event leaked secret: %q", event.Error)
	}
}
