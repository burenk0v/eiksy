package app

import (
	"strings"

	"eiksy/internal/domain/ai"
)

// RecordCommandPolicyResolutionForApp records the authorization outcome for a
// command that entered the human-approval path. Execution itself is audited
// separately by ssh.exec, so this event records only the authorization outcome.
func (s *Service) RecordCommandPolicyResolutionForApp(request ai.CommandRequest, mode ai.CommandPermissionMode, err error) {
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
		"ask",
		approval,
		result,
		0,
		0,
		extra,
	)
}
