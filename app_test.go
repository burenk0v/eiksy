package main

import (
	"testing"
)

func TestOpenRDPFailsForUnknownProfile(t *testing.T) {
	app := NewApp()
	if err := app.OpenRDP("rdp-tab", "unknown-profile"); err == nil {
		t.Fatal("expected error for unknown RDP profile")
	}
}
