package main

import "strings"

const aiSessionContextMarker = "\n\n[Latest console output]\n"

// prepareAIChatMessage keeps the active SSH session available for AI tools only
// when the GUI explicitly opted into session context. The legacy UI checkbox
// currently appends aiSessionContextMarker, so the marker is also used as the
// opt-in signal while the UI is migrated to the new label/semantics.
func prepareAIChatMessage(message, activeSessionID string) (string, string) {
	message = strings.TrimSpace(message)
	activeSessionID = strings.TrimSpace(activeSessionID)
	if activeSessionID == "" {
		return message, ""
	}
	if index := strings.Index(message, aiSessionContextMarker); index >= 0 {
		return strings.TrimSpace(message[:index]), activeSessionID
	}
	return message, ""
}
