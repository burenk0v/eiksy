package settings

import (
	"encoding/json"
	"testing"
)

func TestWindowStateJSONRoundTrip(t *testing.T) {
	input := WindowState{
		Width:     1600,
		Height:    1000,
		X:         -120,
		Y:         40,
		Maximized: false,
		Saved:     true,
	}

	data, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("marshal window state: %v", err)
	}

	var output WindowState
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatalf("unmarshal window state: %v", err)
	}

	if output != input {
		t.Fatalf("window state mismatch: got %#v, want %#v", output, input)
	}
}
