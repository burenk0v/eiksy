package app

import (
	"strings"
	"time"

	"eiksy/internal/domain/ai"
)

// recordCommandPolicyResolution appends the second lifecycle event for a
// command that entered the human-approval path. Execution itself is audited
// separately by ssh.exec, so this event records only the authorization outcome.
func (s *Service) recordCommandPolicyResolution(request ai.CommandRequest, mode ai.CommandPermissionMode, err error) {
	approval := "denied"
	result := "approval_denied"
	extra := ai.CommandAuditEvent{ErrorType: "approval_denied"}

	if mode != ai.CommandPermissionModeDeny {
		approval = "approved"
		result = "approved"
		extra = ai.CommandAuditEvent{}
	}
	if err != nil {
		result = "approval_resolution_failed"
		extra.ErrorType = "approval_resolution_error"
		extra.Error = strings.TrimSpace(err.Error())
	}

	s.recordCommandAudit(
		"",
		request.SessionID,
		request.Command,
		string(commandPolicyDecisionAsk),
		approval,
		result,
		0,
		0,
		extra,
	)
}

// commandAuditLifecycleResult is kept as a small internal helper so tests can
// assert the exact lifecycle mapping without depending on timestamps.
type commandAuditLifecycleResult struct {
	Approval string
	Result   string
	Error    string
}

func commandAuditLifecycleResultForMode(mode ai.CommandPermissionMode, err error) commandAuditLifecycleResult {
	result := commandAuditLifecycleResult{Approval: "denied", Result: "approval_denied"}
	if mode != ai.CommandPermissionModeDeny {
		result = commandAuditLifecycleResult{Approval: "approved", Result: "approved"}
	}
	if err != nil {
		result.Result = "approval_resolution_failed"
		result.Error = strings.TrimSpace(err.Error())
	}
	return result
}

var _ = time.Time{}
