package app

import (
	"encoding/json"
	"testing"
)

func TestCommandToolRegistry(t *testing.T) {
	tools := commandToolRegistry()
	if len(tools) != 4 {
		t.Fatalf("expected four AI tools, got %d", len(tools))
	}
	if tools[0].Name != "ssh.exec" || tools[1].Name != "sftp.write" || tools[2].Name != "sftp.list" || tools[3].Name != "ssh.diagnostics" {
		t.Fatalf("unexpected tool names: %#v", tools)
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
	diagnosticsParams, ok := tools[3].Parameters["properties"].(map[string]any)
	if !ok {
		t.Fatal("diagnostics parameters do not contain properties")
	}
	if _, ok := diagnosticsParams["operation"]; !ok {
		t.Fatal("diagnostics tool is missing operation parameter")
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
	if len(decoded) != 4 || decoded[0]["type"] != "function" || decoded[1]["type"] != "function" || decoded[2]["type"] != "function" || decoded[3]["type"] != "function" {
		t.Fatalf("unexpected OpenAI tool definitions: %#v", decoded)
	}
}
