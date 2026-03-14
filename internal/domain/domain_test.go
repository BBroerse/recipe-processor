package domain_test

import (
	"testing"
	"time"

	"github.com/bbroerse/recipe-processor/internal/domain"
	"github.com/stretchr/testify/assert"
)

func TestRecipeStatus_Constants(t *testing.T) {
	tests := []struct {
		status   domain.RecipeStatus
		expected string
	}{
		{domain.StatusPending, "pending"},
		{domain.StatusProcessing, "processing"},
		{domain.StatusCompleted, "completed"},
		{domain.StatusFailed, "failed"},
	}
	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, string(tt.status))
		})
	}
}

func TestRecipeSubmitted_Event(t *testing.T) {
	now := time.Now().UTC()
	e := domain.RecipeSubmitted{RecipeID: "abc", RawInput: "test", Timestamp: now}

	assert.Equal(t, "recipe.submitted", e.EventType())
	assert.Equal(t, now, e.OccurredAt())
}

func TestRecipeProcessed_Event(t *testing.T) {
	now := time.Now().UTC()
	e := domain.RecipeProcessed{RecipeID: "abc", RawResponse: "{}", Timestamp: now}

	assert.Equal(t, "recipe.processed", e.EventType())
	assert.Equal(t, now, e.OccurredAt())
}

func TestRecipeProcessingFailed_Event(t *testing.T) {
	now := time.Now().UTC()
	e := domain.RecipeProcessingFailed{RecipeID: "abc", Error: "timeout", Timestamp: now}

	assert.Equal(t, "recipe.processing_failed", e.EventType())
	assert.Equal(t, now, e.OccurredAt())
}
