package sse

import (
	"encoding/json"
	"fmt"
	"strings"
)

type Event struct {
	Type   string      // Event type (e.g., "mixer-update", "control-update")
	Data   interface{} // Event data (JSON or HTML string)
	IsHTML bool        // If true, Data is treated as raw HTML; otherwise JSON
	ID     string      // Optional event ID for resuming connections
}

func (e Event) String() string {
	var result string

	if e.ID != "" {
		result += fmt.Sprintf("id: %s\n", e.ID)
	}

	if e.Type != "" {
		result += fmt.Sprintf("event: %s\n", e.Type)
	}

	var dataStr string
	if e.IsHTML {
		dataStr = e.Data.(string)
	} else {
		dataBytes, err := json.Marshal(e.Data)
		if err != nil {
			result += fmt.Sprintf("data: %s\n\n", fmt.Sprintf("error: %v", err))
			return result
		}
		dataStr = string(dataBytes)
	}

	dataStr = strings.ReplaceAll(dataStr, "\r\n", "\n")
	lines := strings.Split(dataStr, "\n")
	for _, line := range lines {
		result += fmt.Sprintf("data: %s\n", line)
	}
	result += "\n"

	return result
}
