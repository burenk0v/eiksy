package app

import (
	"fmt"
	"strings"

	"eiksy/internal/domain/ai"
)

// GetAIInfrastructureContext builds a deliberately non-secret snapshot for AI
// reasoning. It contains only the selected runtime session and safe connection
// metadata; credentials, secret references and connection options are excluded.
func (s *Service) GetAIInfrastructureContext(sessionID string) (ai.InfrastructureContext, error) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return ai.InfrastructureContext{}, nil
	}

	var tab *RuntimeSessionView
	for _, candidate := range s.store.RuntimeTabs() {
		if candidate.ID != sessionID {
			continue
		}
		view := RuntimeSessionView(candidate)
		tab = &view
		break
	}
	if tab == nil {
		return ai.InfrastructureContext{}, fmt.Errorf("active session %q not found", sessionID)
	}

	ctx := ai.SessionContext{
		ID:          tab.ID,
		Name:        tab.Title,
		ProtocolID:  tab.ProtocolID,
		Status:      tab.Status,
		Description: tab.Description,
	}

	if profile, ok := s.store.SessionProfile(tab.ProfileID); ok {
		ctx.Name = profile.Name
		ctx.Group = profile.Group
		ctx.Tags = append([]string(nil), profile.Tags...)
		ctx.Host = profile.Host
		ctx.Port = profile.Port
		ctx.Username = profile.Username
	}

	// CurrentDir is useful operational context, but a failure to obtain it must
	// not make the whole context unavailable. GetCurrentDir is read-only from the
	// AI perspective and never exposes command output or credentials.
	if s.sshManager != nil {
		if currentDir, err := s.sshManager.GetCurrentDir(sessionID); err == nil {
			ctx.CurrentDir = strings.TrimSpace(currentDir)
		}
	}

	return ai.InfrastructureContext{ActiveSession: &ctx}, nil
}
