package app

import (
	"fmt"
	"strings"

	"eiksy/internal/domain/protocols"
	"eiksy/internal/domain/resources"
	"eiksy/internal/domain/workspace"
)

// GetResources returns configured operational targets without exposing
// credentials or secret references. A resource is the stable product-level
// identity used by Connect and later by Operate.
func (s *Service) GetResources() []resources.Resource {
	protocolsByID := make(map[string]protocols.Descriptor)
	for _, descriptor := range s.store.Protocols() {
		protocolsByID[descriptor.ID] = descriptor
	}

	activeByProfile := make(map[string]workspaceTabResourceState)
	for _, tab := range s.store.RuntimeTabs() {
		activeByProfile[tab.ProfileID] = workspaceTabResourceState{
			id:     tab.ID,
			status: tab.Status,
		}
	}

	profiles := s.store.SessionProfiles()
	result := make([]resources.Resource, 0, len(profiles))
	for _, profile := range profiles {
		descriptor := protocolsByID[profile.ProtocolID]
		state := activeByProfile[profile.ID]
		status := "configured"
		if state.id != "" {
			status = state.status
		}

		result = append(result, resources.Resource{
			ID:              profile.ID,
			Name:            strings.TrimSpace(profile.Name),
			Group:           strings.TrimSpace(profile.Group),
			Tags:            append([]string(nil), profile.Tags...),
			Kind:            "remote",
			ProtocolID:      profile.ProtocolID,
			Host:            profile.Host,
			Port:            profile.Port,
			Username:        profile.Username,
			Capabilities:    append([]protocols.Capability(nil), descriptor.Capabilities...),
			Status:          status,
			ActiveSessionID: state.id,
		})
	}
	return result
}

type workspaceTabResourceState struct {
	id     string
	status string
}

func (s *Service) GetResource(resourceID string) (resources.Resource, error) {
	resourceID = strings.TrimSpace(resourceID)
	if resourceID == "" {
		return resources.Resource{}, fmt.Errorf("resource id is required")
	}
	for _, resource := range s.GetResources() {
		if resource.ID == resourceID {
			return resource, nil
		}
	}
	return resources.Resource{}, fmt.Errorf("resource %q not found", resourceID)
}
