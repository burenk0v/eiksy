package sshmanager

import "testing"

func TestDisconnectClearsPerTabState(t *testing.T) {
	m := NewManager()
	m.handlers["tab-1"] = func(string) {}
	m.pendingKeys["tab-1"] = &PendingHostKey{Hostname: "host"}

	if err := m.Disconnect("tab-1"); err != nil {
		t.Fatalf("disconnect: %v", err)
	}

	if _, ok := m.handlers["tab-1"]; ok {
		t.Fatal("expected output handler to be removed")
	}
	if _, ok := m.pendingKeys["tab-1"]; ok {
		t.Fatal("expected pending host key to be removed")
	}
}

func TestStaleSessionCannotDisconnectReconnectedTab(t *testing.T) {
	m := NewManager()
	oldConn := &connection{}
	newConn := &connection{}
	m.connections["tab-1"] = newConn

	if err := m.disconnectConnection("tab-1", oldConn); err != nil {
		t.Fatalf("stale disconnect: %v", err)
	}

	if m.connections["tab-1"] != newConn {
		t.Fatal("stale session disconnected the active replacement connection")
	}
}

func TestActiveSessionDisconnectRemovesConnection(t *testing.T) {
	m := NewManager()
	active := &connection{}
	m.connections["tab-1"] = active
	m.handlers["tab-1"] = func(string) {}
	m.pendingKeys["tab-1"] = &PendingHostKey{Hostname: "host"}

	if err := m.disconnectConnection("tab-1", active); err != nil {
		t.Fatalf("active disconnect: %v", err)
	}

	if _, ok := m.connections["tab-1"]; ok {
		t.Fatal("expected active connection to be removed")
	}
	if _, ok := m.handlers["tab-1"]; ok {
		t.Fatal("expected output handler to be removed")
	}
	if _, ok := m.pendingKeys["tab-1"]; ok {
		t.Fatal("expected pending host key to be removed")
	}
}
