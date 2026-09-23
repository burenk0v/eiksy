package debuglog

import "testing"

func TestPathIsEmptyWhenDebugLoggingIsDisabled(t *testing.T) {
	if got := Path(); got != "" {
		t.Fatalf("expected empty debug log path, got %q", got)
	}
}
