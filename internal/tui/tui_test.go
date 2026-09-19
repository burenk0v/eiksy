package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestModelHandlesWindowSize(t *testing.T) {
	m := NewModel()
	next, cmd := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	if cmd != nil {
		t.Fatal("expected no command")
	}

	got := next.(Model)
	if got.width != 120 || got.height != 40 {
		t.Fatalf("expected 120x40, got %dx%d", got.width, got.height)
	}
}

func TestModelQuitsOnQ(t *testing.T) {
	m := NewModel()
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatal("expected quit command")
	}
}

func TestModelHasInitialTabs(t *testing.T) {
	m := NewModel()
	want := []string{"Chat", "Terminal", "Files", "Tools"}
	if len(m.tabs) != len(want) {
		t.Fatalf("expected %d tabs, got %d", len(want), len(m.tabs))
	}
	for i, title := range want {
		if m.tabs[i].Title != title {
			t.Fatalf("expected tab %d to be %q, got %q", i, title, m.tabs[i].Title)
		}
	}
	if m.activeTab != 0 {
		t.Fatalf("expected initial tab 0, got %d", m.activeTab)
	}
}

func TestModelNavigatesTabs(t *testing.T) {
	m := NewModel()

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	if cmd != nil {
		t.Fatal("expected no command")
	}
	m = next.(Model)
	if m.activeTab != 1 {
		t.Fatalf("expected active tab 1, got %d", m.activeTab)
	}

	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	m = next.(Model)
	if m.activeTab != 0 {
		t.Fatalf("expected active tab 0, got %d", m.activeTab)
	}

	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	m = next.(Model)
	if m.activeTab != len(m.tabs)-1 {
		t.Fatalf("expected wrap to last tab, got %d", m.activeTab)
	}
}

func TestModelViewContainsTabs(t *testing.T) {
	m := NewModel()
	view := m.View()
	for _, want := range []string{"EIKSY", "Think. Connect. Operate.", "[Chat]", "Terminal", "Files", "Tools", "q quit"} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected view to contain %q, got %q", want, view)
		}
	}
}

func TestTabSessionReference(t *testing.T) {
	tab := Tab{Title: "Chat"}

	tab.BindSession("session-1", "server-01")
	if tab.Session == nil {
		t.Fatal("expected session reference")
	}
	if tab.Session.ID != "session-1" || tab.Session.Title != "server-01" {
		t.Fatalf("unexpected session reference: %+v", tab.Session)
	}

	tab.UnbindSession()
	if tab.Session != nil {
		t.Fatal("expected session reference to be cleared")
	}
}

func TestTabBindSessionIgnoresEmptyID(t *testing.T) {
	tab := Tab{Title: "Chat"}
	tab.BindSession("  ", "server-01")
	if tab.Session != nil {
		t.Fatal("expected empty session ID not to create a reference")
	}
}
