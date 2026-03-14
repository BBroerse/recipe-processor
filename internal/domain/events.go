package domain

import "time"

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

func (e RecipeSubmitted) EventType() string    { return "recipe.submitted" }
func (e RecipeSubmitted) OccurredAt() time.Time { return e.Timestamp }

// RecipeProcessed is published after the LLM successfully parses a recipe.
type RecipeProcessed struct {
	RecipeID    string
	Timestamp   time.Time
	RawResponse string
}

func (e RecipeProcessed) EventType() string    { return "recipe.processed" }
func (e RecipeProcessed) OccurredAt() time.Time { return e.Timestamp }

// RecipeProcessingFailed is published when LLM processing fails or returns unusable data.
type RecipeProcessingFailed struct {
	RecipeID  string
	Timestamp time.Time
	Error     string
}

func (e RecipeProcessingFailed) EventType() string    { return "recipe.processing_failed" }
func (e RecipeProcessingFailed) OccurredAt() time.Time { return e.Timestamp }
