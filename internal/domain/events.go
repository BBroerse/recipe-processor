package domain

import "time"

// Event is the interface for all domain events.
type Event interface {
	EventType() string
	OccurredAt() time.Time
}

// RecipeSubmitted is published when a user submits raw recipe text for processing.
type RecipeSubmitted struct {
	RecipeID  string
	Timestamp time.Time
	RawInput  string
}

// EventType returns the event type identifier.
func (e RecipeSubmitted) EventType() string { return "recipe.submitted" }

// OccurredAt returns the timestamp of the event.
func (e RecipeSubmitted) OccurredAt() time.Time { return e.Timestamp }

// RecipeProcessed is published when the LLM successfully processes a recipe.
type RecipeProcessed struct {
	RecipeID    string
	Timestamp   time.Time
	RawResponse string
}

// EventType returns the event type identifier.
func (e RecipeProcessed) EventType() string { return "recipe.processed" }

// OccurredAt returns the timestamp of the event.
func (e RecipeProcessed) OccurredAt() time.Time { return e.Timestamp }

// RecipeProcessingFailed is published when the LLM fails to process a recipe.
type RecipeProcessingFailed struct {
	RecipeID  string
	Timestamp time.Time
	Error     string
}

// EventType returns the event type identifier.
func (e RecipeProcessingFailed) EventType() string { return "recipe.processing_failed" }

// OccurredAt returns the timestamp of the event.
func (e RecipeProcessingFailed) OccurredAt() time.Time { return e.Timestamp }
