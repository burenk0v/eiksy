package app

import (
	"fmt"
	"strings"

	"eiksy/internal/domain/ai"
)

// GetAIInfrastructureContext builds a deliberately non-secret snapshot for AI
// reasoning. It contains only the selected runtime session and safe connection
// metadata; credentials, secret references and connection options are excluded.
const (
	maxAIContextName        = 256
	maxAIContextDescription = 512
	maxAIContextHost        = 255
	maxAIContextUsername    = 128
	maxAIContextGroup       = 128
	maxAIContextTag         = 64
	maxAIContextTags        = 32
	maxAIContextCurrentDir  = 1024
)

func boundAIContextString(value string, max int) string {
	value = strings.TrimSpace(value)
	if len(value) <= max {
		return value
	}
	return value[:max]
}

func boundAIContextTags(tags []string) []string {
	result := make([]string, 0, min(len(tags), maxAIContextTags))
	for _, tag := range tags {
		tag = boundAIContextString(tag, maxAIContextTag)
		if tag == "" {
			continue
		}
		result = append(result, tag)
		if len(result) == maxAIContextTags {
			break
		}
	}
	return result
}

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
		Description: boundAIContextString(tab.Description, maxAIContextDescription),
	}

	if profile, ok := s.store.SessionProfile(tab.ProfileID); ok {
		ctx.Name = boundAIContextString(profile.Name, maxAIContextName)
		ctx.Group = boundAIContextString(profile.Group, maxAIContextGroup)
		ctx.Tags = boundAIContextTags(profile.Tags)
		ctx.Host = boundAIContextString(profile.Host, maxAIContextHost)
		ctx.Port = profile.Port
		ctx.Username = boundAIContextString(profile.Username, maxAIContextUsername)
	}

	// CurrentDir is useful operational context, but a failure to obtain it must
	// not make the whole context unavailable. GetCurrentDir is read-only from the
	// AI perspective and never exposes command output or credentials.
	if s.sshManager != nil {
		if currentDir, err := s.sshManager.GetCurrentDir(sessionID); err == nil {
			ctx.CurrentDir = boundAIContextString(currentDir, maxAIContextCurrentDir)
		}
	}

	return ai.InfrastructureContext{ActiveSession: &ctx}, nil
}
