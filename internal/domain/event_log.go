package domain

import "time"

// EventLogEntry is a persisted record of a domain event for debugging and audit.
type EventLogEntry struct {
	ID        string
	EventType string
	RecipeID  string
	Payload   string // JSON-encoded event data
	CreatedAt time.Time
}
