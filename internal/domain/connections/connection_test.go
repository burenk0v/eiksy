package connections

import (
	"context"
	"testing"
)

type fakeConnection struct{ connected bool }

func (f *fakeConnection) Connect(context.Context, Profile) error { f.connected = true; return nil }
func (f *fakeConnection) Disconnect(context.Context) error        { f.connected = false; return nil }
func (f *fakeConnection) Connected() bool                          { return f.connected }

func TestConnectionLifecycleContract(t *testing.T) {
	var conn Connection = &fakeConnection{}
	profile := Profile{ID: "session-1", Protocol: ProtocolSSH, Host: "example.test", Port: 22}

	if conn.Connected() {
		t.Fatal("connection must start disconnected")
	}
	if err := conn.Connect(context.Background(), profile); err != nil {
		t.Fatalf("connect: %v", err)
	}
	if !conn.Connected() {
		t.Fatal("connection must report connected after Connect")
	}
	if err := conn.Disconnect(context.Background()); err != nil {
		t.Fatalf("disconnect: %v", err)
	}
	if conn.Connected() {
		t.Fatal("connection must report disconnected after Disconnect")
	}
}
