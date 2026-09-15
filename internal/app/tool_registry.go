package app

import "encoding/json"

// AITool describes an action that the model may request. The model never gets
direct access to the implementation; Service remains the execution gate.
type AITool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

// commandToolRegistry is deliberately small. New capabilities should be added
// here before they are exposed to a local model.
func commandToolRegistry() []AITool {
	return []AITool{
		{
			Name: "ssh.exec",
			Description: "Execute one command in the selected active SSH session. The command is always subject to Eiksy Command Policy and may require user approval.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"sessionId": map[string]any{"type": "string", "description": "Active Eiksy SSH session ID."},
					"command":   map[string]any{"type": "string", "description": "One shell command to execute. Do not combine commands with shell operators."},
					"reason":    map[string]any{"type": "string", "description": "Short explanation of why the command is needed."},
				},
				"required": []string{"sessionId", "command"},
				"additionalProperties": false,
			},
		},
		{
			Name: "ssh.diagnostics",
			Description: "Run a fixed, read-only infrastructure diagnostic summary against the selected active SSH session. No arbitrary command is accepted.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"sessionId": map[string]any{"type": "string", "description": "Active Eiksy SSH session ID."},
					"operation": map[string]any{"type": "string", "enum": []string{"summary"}, "description": "Diagnostic operation to run."},
				},
				"required": []string{"sessionId", "operation"},
				"additionalProperties": false,
			},
		},
	}
}

// openAIToolDefinitions converts the internal registry into the OpenAI
// compatible tools representation used by Ollama/llama-server and compatible
// local providers.
func openAIToolDefinitions() []map[string]any {
	registry := commandToolRegistry()
	tools := make([]map[string]any, 0, len(registry))
	for _, tool := range registry {
		tools = append(tools, map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        tool.Name,
				"description": tool.Description,
				"parameters":  tool.Parameters,
			},
		})
	}
	return tools
}

func marshalAITools() ([]byte, error) {
	return json.Marshal(openAIToolDefinitions())
}
