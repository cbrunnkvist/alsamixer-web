package sse

import (
	"encoding/json"
	"fmt"
)

// Event represents a Server-Sent Event with type, data, and optional ID.
type Event struct {
	Type string      // Event type (e.g., "mixer-update", "volume-change")
	Data interface{} // Event data (will be JSON-encoded)
	ID   string      // Optional event ID for resuming connections
}

// String formats the event according to the SSE specification.
// Format: "event: type\ndata: json\n\n" (with optional id field)
func (e Event) String() string {
	var result string

	if e.ID != "" {
		result += fmt.Sprintf("id: %s\n", e.ID)
	}

	if e.Type != "" {
		result += fmt.Sprintf("event: %s\n", e.Type)
	}

	dataBytes, err := json.Marshal(e.Data)
	if err != nil {
		result += fmt.Sprintf("data: %s\n\n", fmt.Sprintf("error: %v", err))
		return result
	}

	result += fmt.Sprintf("data: %s\n\n", string(dataBytes))

	return result
}
