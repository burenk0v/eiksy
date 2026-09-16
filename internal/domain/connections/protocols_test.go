package connections

import "testing"

func TestSupportedProtocols(t *testing.T) {
	want := []Protocol{ProtocolSSH, ProtocolSFTP, ProtocolRDP, ProtocolLocal}
	if len(SupportedProtocols) != len(want) {
		t.Fatalf("got %d protocols, want %d", len(SupportedProtocols), len(want))
	}
	for i, protocol := range want {
		if SupportedProtocols[i] != protocol {
			t.Fatalf("protocol %d = %q, want %q", i, SupportedProtocols[i], protocol)
		}
	}
}
