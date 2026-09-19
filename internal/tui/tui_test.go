package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestModelHandlesWindowSize(t *testing.T) {
	m := NewModel()
	next, cmd := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	if cmd != nil { t.Fatal("expected no command") }

	got := next.(Model)
	if got.width != 120 || got.height != 40 {
		t.Fatalf("expected 120x40, got %dx%d", got.width, got.height)
	}
}

func TestModelQuitsOnQ(t *testing.T) {
	m := NewModel()
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil { t.Fatal("expected quit command") }
}

func TestModelViewContainsFoundationLayout(t *testing.T) {
	m := NewModel()
	view := m.View()
	for _, want := range []string{"EIKSY", "Think. Connect. Operate.", "TUI foundation", "q quit"} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected view to contain %q, got %q", want, view)
		}
	}
}
