package domain

import "time"

type Event interface {
	EventType() string
	OccurredAt() time.Time
}

type RecipeSubmitted struct {
	RecipeID  string
	Timestamp time.Time
	RawInput  string
}

func (e RecipeSubmitted) EventType() string    { return "recipe.submitted" }
func (e RecipeSubmitted) OccurredAt() time.Time { return e.Timestamp }

type RecipeProcessed struct {
	RecipeID    string
	Timestamp   time.Time
	RawResponse string
}

func (e RecipeProcessed) EventType() string    { return "recipe.processed" }
func (e RecipeProcessed) OccurredAt() time.Time { return e.Timestamp }

type RecipeProcessingFailed struct {
	RecipeID  string
	Timestamp time.Time
	Error     string
}

func (e RecipeProcessingFailed) EventType() string    { return "recipe.processing_failed" }
func (e RecipeProcessingFailed) OccurredAt() time.Time { return e.Timestamp }
