package sshmanager

import "testing"

func TestParseProxyJumpsPreservesCompleteChain(t *testing.T) {
	got := parseProxyJumps("alice@jump-a:22,jump-b:2200,bob@jump-c", "target")
	if len(got) != 3 { t.Fatalf("expected 3 proxy jumps, got %d", len(got)) }
	if got[0].User != "alice" || got[0].Address != "jump-a:22" { t.Fatalf("unexpected first jump: %+v", got[0]) }
	if got[1].User != "target" || got[1].Address != "jump-b:2200" { t.Fatalf("unexpected second jump: %+v", got[1]) }
	if got[2].User != "bob" || got[2].Address != "jump-c:22" { t.Fatalf("unexpected third jump: %+v", got[2]) }
}
