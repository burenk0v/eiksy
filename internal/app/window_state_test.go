package app

import (
	"testing"

	"eiksy/internal/domain/settings"
	"eiksy/internal/storage/memory"
)

func TestUpdateSettingsPreservesSavedWindowState(t *testing.T) {
	store := memory.NewStore()
	service := NewService(store, nil, nil)

	initial := store.Settings()
	initial.WindowState = settings.WindowState{
		Width:     1600,
		Height:    1000,
		X:         120,
		Y:         80,
		Maximized: false,
		Saved:     true,
	}
	if err := store.UpdateSettings(initial); err != nil {
		t.Fatalf("seed settings: %v", err)
	}

	updated := store.Settings()
	updated.Theme = "light"
	updated.WindowState = settings.WindowState{}
	if err := service.UpdateSettings(updated); err != nil {
		t.Fatalf("update settings: %v", err)
	}

	got := store.Settings().WindowState
	if got != initial.WindowState {
		t.Fatalf("window state changed: got %#v, want %#v", got, initial.WindowState)
	}
}
