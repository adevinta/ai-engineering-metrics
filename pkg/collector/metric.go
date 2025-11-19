package collector

import "time"

// Metric represents a single AI usage metric
type Metric struct {
	UserID   string         `json:"user_id"`
	From     time.Time      `json:"from"`
	To       time.Time      `json:"to"`
	ToolName string         `json:"tool_name"`
	Metrics  map[string]any `json:"metrics"`
}
