package main

import (
	"context"

	"opsy/internal/app"
	"opsy/internal/storage/memory"
)

// App wires the Wails bridge to backend services.
type App struct {
	ctx     context.Context
	service *app.Service
}

// NewApp creates the root application instance.
func NewApp() *App {
	store := memory.NewStore()

	return &App{
		service: app.NewService(store),
	}
}

// startup is called when the app starts.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// GetShellState returns the full backend-owned application state for the UI shell.
func (a *App) GetShellState() app.ShellState {
	return a.service.GetShellState()
}

// LaunchSession opens a new runtime tab from a saved session profile.
func (a *App) LaunchSession(profileID string) (app.RuntimeSessionView, error) {
	return a.service.LaunchSession(profileID)
}

// CloseSession closes an active runtime tab.
func (a *App) CloseSession(sessionID string) error {
	return a.service.CloseSession(sessionID)
}
