package sftpmanager

import (
	"os"
	"path/filepath"
	"testing"
)

func TestKnownHostsPathUsesUserSSHDirectory(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	path, err := knownHostsPath()
	if err != nil {
		t.Fatalf("knownHostsPath() error = %v", err)
	}
	want := filepath.Join(home, ".ssh", "known_hosts")
	if path != want {
		t.Fatalf("knownHostsPath() = %q, want %q", path, want)
	}
}

func TestHostKeyCallbackRequiresKnownHostsFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if _, err := hostKeyCallback(); err == nil {
		t.Fatal("hostKeyCallback() error = nil, want missing known_hosts error")
	}

	sshDir := filepath.Join(home, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sshDir, "known_hosts"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := hostKeyCallback(); err != nil {
		t.Fatalf("hostKeyCallback() with known_hosts error = %v", err)
	}
}
