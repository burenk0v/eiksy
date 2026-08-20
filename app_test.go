package main

import (
	"slices"
	"testing"
)

func TestRDPLaunchCommandUsesMstscOnWindows(t *testing.T) {
	command, args, ok := rdpLaunchCommand("rdp://windows-host.example.com:3389", "windows")
	if !ok {
		t.Fatal("expected windows rdp launch command")
	}
	if command != "mstsc" {
		t.Fatalf("expected mstsc command, got %q", command)
	}
	if !slices.Equal(args, []string{"/v:windows-host.example.com:3389"}) {
		t.Fatalf("unexpected args: %#v", args)
	}
}

func TestRDPLaunchCommandSkipsUnsupportedTargets(t *testing.T) {
	if _, _, ok := rdpLaunchCommand("rdp://linux-host.example.com:3389", "linux"); ok {
		t.Fatal("expected non-windows platforms to skip native mstsc launch")
	}
	if _, _, ok := rdpLaunchCommand("not-a-url", "windows"); ok {
		t.Fatal("expected invalid targets to skip native mstsc launch")
	}
}
