package cli

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestRunWithoutArgumentsStartsDesktopApp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	handled, exitCode := Run(nil, "v1.2.3", &stdout, &stderr, func(TUIConfig) error { return nil })
	if handled {
		t.Fatal("expected no-argument invocation to start the desktop app")
	}
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}
}

func TestRunHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	handled, exitCode := Run([]string{"--help"}, "v1.2.3", &stdout, &stderr, func() error { return nil })
	if !handled || exitCode != 0 {
		t.Fatalf("expected handled success, got handled=%v exitCode=%d", handled, exitCode)
	}
	if !strings.Contains(stdout.String(), "Usage:") {
		t.Fatalf("expected usage in help output, got %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}
}

func TestRunVersion(t *testing.T) {
	var stdout, stderr bytes.Buffer
	handled, exitCode := Run([]string{"--version"}, "v1.2.3", &stdout, &stderr, func() error { return nil })
	if !handled || exitCode != 0 {
		t.Fatalf("expected handled success, got handled=%v exitCode=%d", handled, exitCode)
	}
	if stdout.String() != "v1.2.3\n" {
		t.Fatalf("expected version output, got %q", stdout.String())
	}
}

func TestRunTUI(t *testing.T) {
	var stdout, stderr bytes.Buffer
	called := false
	handled, exitCode := Run([]string{"tui"}, "v1.2.3", &stdout, &stderr, func() error {
		called = true
		return nil
	})
	if !handled || exitCode != 0 || !called {
		t.Fatalf("expected TUI to run successfully, got handled=%v exitCode=%d called=%v", handled, exitCode, called)
	}
}

func TestRunTUIError(t *testing.T) {
	var stdout, stderr bytes.Buffer
	handled, exitCode := Run([]string{"tui"}, "v1.2.3", &stdout, &stderr, func() error {
		return errors.New("terminal unavailable")
	})
	if !handled || exitCode != 1 {
		t.Fatalf("expected TUI failure exit code 1, got handled=%v exitCode=%d", handled, exitCode)
	}
	if !strings.Contains(stderr.String(), "terminal unavailable") {
		t.Fatalf("expected TUI error, got %q", stderr.String())
	}
}

func TestRunUnknownArgument(t *testing.T) {
	var stdout, stderr bytes.Buffer
	handled, exitCode := Run([]string{"--unknown"}, "v1.2.3", &stdout, &stderr, func() error { return nil })
	if !handled || exitCode != 2 {
		t.Fatalf("expected handled error, got handled=%v exitCode=%d", handled, exitCode)
	}
	if !strings.Contains(stderr.String(), "unknown command or option") {
		t.Fatalf("expected unknown-option error, got %q", stderr.String())
	}
}


func TestRunTUIConfiguration(t *testing.T) {
	var stdout, stderr bytes.Buffer
	var got TUIConfig
	handled, exitCode := Run([]string{"tui", "--no-alt-screen", "--view", "files"}, "v1.2.3", &stdout, &stderr, func(config TUIConfig) error {
		got = config
		return nil
	})
	if !handled || exitCode != 0 {
		t.Fatalf("expected handled success, got handled=%v exitCode=%d", handled, exitCode)
	}
	if got.AltScreen || got.InitialView != "files" {
		t.Fatalf("unexpected TUI config: %+v", got)
	}
}

func TestRunTUIRejectsInvalidConfiguration(t *testing.T) {
	var stdout, stderr bytes.Buffer
	handled, exitCode := Run([]string{"tui", "--view", "unknown"}, "v1.2.3", &stdout, &stderr, func(TUIConfig) error { return nil })
	if !handled || exitCode != 2 {
		t.Fatalf("expected handled configuration error, got handled=%v exitCode=%d", handled, exitCode)
	}
	if !strings.Contains(stderr.String(), "invalid --view") {
		t.Fatalf("expected invalid-view error, got %q", stderr.String())
	}
}
