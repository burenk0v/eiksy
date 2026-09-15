package main

import (
	"eiksy/internal/domain/ai"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// GetAIInfrastructureContext exposes a safe, backend-built infrastructure
// snapshot to the UI. The snapshot intentionally excludes credential material.
func (a *App) GetAIInfrastructureContext(sessionID string) (ai.InfrastructureContext, error) {
	return a.currentService().GetAIInfrastructureContext(sessionID)
}

// RefreshAIInfrastructureContext emits the current context for UI consumers
// that prefer event-driven updates.
func (a *App) RefreshAIInfrastructureContext(sessionID string) error {
	ctx, err := a.currentService().GetAIInfrastructureContext(sessionID)
	if err != nil {
		return err
	}
	runtime.EventsEmit(a.ctx, "ai:infrastructure-context", ctx)
	return nil
}
