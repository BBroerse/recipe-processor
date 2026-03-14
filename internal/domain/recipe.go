package domain

import "time"

// Recipe is the core entity representing a recipe processing request and its result.
type Recipe struct {
	ID        string       `json:"id"`
	RawInput  string       `json:"raw_input"`
	Status    RecipeStatus `json:"status"`
	CreatedAt time.Time    `json:"created_at"`
	UpdatedAt time.Time    `json:"updated_at"`

	// Structured fields populated after LLM processing
	Title        string   `json:"title"`
	Ingredients  []string `json:"ingredients"`
	Instructions []string `json:"instructions"`
	TotalTime    int      `json:"total_time"`
	Servings     int      `json:"servings"`
	CourseType   string   `json:"course_type"`

	// Raw LLM response preserved for debugging/reprocessing
	RawResponse string `json:"raw_response,omitempty"`
}

type RecipeStatus string

const (
	StatusPending    RecipeStatus = "pending"
	StatusProcessing RecipeStatus = "processing"
	StatusCompleted  RecipeStatus = "completed"
	StatusFailed     RecipeStatus = "failed"
)
