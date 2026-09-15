package main

import (
	"context"

	"eiksy/internal/app"
	"eiksy/internal/domain/ai"
)

// App is the Wails-facing application facade.
type App struct {
	ctx context.Context
	service *app.Service
}

func (a *App) ResolveCommandPolicyRequest(requestID string, mode string) error {
	service := a.currentService()
	state := service.GetShellState().AI
	var request ai.CommandRequest
	for _, pending := range state.CommandPolicy.PendingRequests {
		if pending.ID == requestID {
			request = pending
			break
		}
	}

	permissionMode := ai.CommandPermissionMode(mode)
	err := service.ResolveCommandPolicyRequest(requestID, permissionMode)
	if request.ID != "" {
		service.RecordCommandPolicyResolutionForApp(request, permissionMode, err)
	}
	return err
}

func (a *App) GetCommandAuditTrail() []ai.CommandAuditEvent {
	return a.currentService().GetCommandAuditTrail()
}
