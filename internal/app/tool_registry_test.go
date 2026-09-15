package app

import (
	"encoding/json"
	"testing"
)

func TestCommandToolRegistry(t *testing.T) {
	tools := commandToolRegistry()
	if len(tools) != 1 {
		t.Fatalf("expected one initial AI tool, got %d", len(tools))
	}
	if tools[0].Name != "ssh.exec" {
		t.Fatalf("unexpected tool name %q", tools[0].Name)
	}
	params, ok := tools[0].Parameters["properties"].(map[string]any)
	if !ok {
		t.Fatal("tool parameters do not contain properties")
	}
	for _, required := range []string{"sessionId", "command"} {
		if _, ok := params[required]; !ok {
			t.Fatalf("missing parameter %q", required)
		}
	}
}

func TestOpenAIToolDefinitionsAreValidJSON(t *testing.T) {
	payload, err := marshalAITools()
	if err != nil {
		t.Fatalf("marshal tools: %v", err)
	}
	var decoded []map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("unmarshal tools: %v", err)
	}
	if len(decoded) != 1 || decoded[0]["type"] != "function" {
		t.Fatalf("unexpected OpenAI tool definition: %#v", decoded)
	}
}
