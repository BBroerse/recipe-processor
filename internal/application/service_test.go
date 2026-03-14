package application_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/bbroerse/recipe-processor/internal/application"
	"github.com/bbroerse/recipe-processor/internal/domain"
	"github.com/bbroerse/recipe-processor/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestService() (*application.RecipeService, *testutil.MockRecipeRepository, *testutil.MockLLMProvider, *testutil.MockEventBus) {
	repo := testutil.NewMockRecipeRepository()
	llm := &testutil.MockLLMProvider{}
	bus := testutil.NewMockEventBus()
	svc := application.NewRecipeService(repo, llm, bus)
	return svc, repo, llm, bus
}

func TestSubmitRecipe_Success(t *testing.T) {
	svc, repo, _, bus := newTestService()

	id, err := svc.SubmitRecipe(context.Background(), "Mix flour and water")
	require.NoError(t, err)
	assert.NotEmpty(t, id)

	// Recipe saved with pending status
	recipe := repo.Recipes[id]
	assert.Equal(t, domain.StatusPending, recipe.Status)
	assert.Equal(t, "Mix flour and water", recipe.RawInput)

	// RecipeSubmitted event published
	events := bus.Events()
	require.Len(t, events, 1)
	submitted, ok := events[0].(domain.RecipeSubmitted)
	assert.True(t, ok)
	assert.Equal(t, id, submitted.RecipeID)
}

func TestSubmitRecipe_EmptyText(t *testing.T) {
	svc, _, _, _ := newTestService()

	_, err := svc.SubmitRecipe(context.Background(), "")
	assert.ErrorIs(t, err, domain.ErrEmptyRecipeText)
}

func TestSubmitRecipe_TextTooLong(t *testing.T) {
	svc, _, _, _ := newTestService()

	longText := strings.Repeat("a", 10_001)
	_, err := svc.SubmitRecipe(context.Background(), longText)
	assert.ErrorIs(t, err, domain.ErrTextTooLong)
}

func TestSubmitRecipe_RepoError(t *testing.T) {
	svc, repo, _, _ := newTestService()
	repo.SaveFunc = func(_ context.Context, _ *domain.Recipe) error {
		return errors.New("db down")
	}

	_, err := svc.SubmitRecipe(context.Background(), "Some recipe")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "saving recipe")
}

func TestGetRecipe_Success(t *testing.T) {
	svc, repo, _, _ := newTestService()
	repo.Recipes["test-id"] = &domain.Recipe{ID: "test-id", Title: "Pasta"}

	recipe, err := svc.GetRecipe(context.Background(), "test-id")
	require.NoError(t, err)
	assert.Equal(t, "Pasta", recipe.Title)
}

func TestGetRecipe_NotFound(t *testing.T) {
	svc, _, _, _ := newTestService()

	_, err := svc.GetRecipe(context.Background(), "nonexistent")
	assert.Error(t, err)
}

func TestHandleRecipeSubmitted_Success(t *testing.T) {
	svc, repo, llm, bus := newTestService()

	// Pre-save recipe
	repo.Recipes["r1"] = &domain.Recipe{ID: "r1", RawInput: "test", Status: domain.StatusPending}

	llmResponse := `{"title":"Pancakes","ingredients":["flour","milk","eggs"],"instructions":["mix","cook"],"total_time":30,"servings":4,"course_type":"main"}`
	llm.ProcessFunc = func(_ context.Context, _ string) (string, error) {
		return llmResponse, nil
	}

	event := domain.RecipeSubmitted{RecipeID: "r1", RawInput: "test", Timestamp: time.Now()}
	err := svc.HandleRecipeSubmitted(context.Background(), event)
	require.NoError(t, err)

	// Recipe updated with structured data
	recipe := repo.Recipes["r1"]
	assert.Equal(t, domain.StatusCompleted, recipe.Status)
	assert.Equal(t, "Pancakes", recipe.Title)
	assert.Equal(t, []string{"flour", "milk", "eggs"}, recipe.Ingredients)
	assert.Equal(t, []string{"mix", "cook"}, recipe.Instructions)
	assert.Equal(t, 30, recipe.TotalTime)
	assert.Equal(t, 4, recipe.Servings)
	assert.Equal(t, "main", recipe.CourseType)

	// RecipeProcessed event published
	events := bus.Events()
	require.Len(t, events, 1)
	_, ok := events[0].(domain.RecipeProcessed)
	assert.True(t, ok)
}

func TestHandleRecipeSubmitted_LLMError(t *testing.T) {
	svc, repo, llm, bus := newTestService()
	repo.Recipes["r1"] = &domain.Recipe{ID: "r1", Status: domain.StatusPending}

	llm.ProcessFunc = func(_ context.Context, _ string) (string, error) {
		return "", errors.New("ollama timeout")
	}

	event := domain.RecipeSubmitted{RecipeID: "r1", RawInput: "test", Timestamp: time.Now()}
	err := svc.HandleRecipeSubmitted(context.Background(), event)
	assert.Error(t, err)

	// Status set to failed
	assert.Equal(t, domain.StatusFailed, repo.Recipes["r1"].Status)

	// RecipeProcessingFailed event published
	events := bus.Events()
	require.Len(t, events, 1)
	failed, ok := events[0].(domain.RecipeProcessingFailed)
	assert.True(t, ok)
	assert.Contains(t, failed.Error, "ollama timeout")
}

func TestHandleRecipeSubmitted_InvalidJSON(t *testing.T) {
	svc, repo, llm, _ := newTestService()
	repo.Recipes["r1"] = &domain.Recipe{ID: "r1", Status: domain.StatusPending}

	llm.ProcessFunc = func(_ context.Context, _ string) (string, error) {
		return "not valid json", nil
	}

	event := domain.RecipeSubmitted{RecipeID: "r1", RawInput: "test", Timestamp: time.Now()}
	err := svc.HandleRecipeSubmitted(context.Background(), event)
	require.NoError(t, err)

	// Raw response saved even if JSON parsing fails
	recipe := repo.Recipes["r1"]
	assert.Equal(t, domain.StatusCompleted, recipe.Status)
	assert.Equal(t, "not valid json", recipe.RawResponse)
	assert.Empty(t, recipe.Title) // structured fields empty
}

func TestHandleRecipeSubmitted_WrongEventType(t *testing.T) {
	svc, _, _, _ := newTestService()

	err := svc.HandleRecipeSubmitted(context.Background(), domain.RecipeProcessed{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected event type")
}

func TestHandleRecipeSubmitted_PartialJSON(t *testing.T) {
	svc, repo, llm, _ := newTestService()
	repo.Recipes["r1"] = &domain.Recipe{ID: "r1", Status: domain.StatusPending}

	// LLM returns JSON with only some fields
	partial := map[string]any{"title": "Soup", "servings": 2}
	data, _ := json.Marshal(partial)
	llm.ProcessFunc = func(_ context.Context, _ string) (string, error) {
		return string(data), nil
	}

	event := domain.RecipeSubmitted{RecipeID: "r1", RawInput: "test", Timestamp: time.Now()}
	err := svc.HandleRecipeSubmitted(context.Background(), event)
	require.NoError(t, err)

	recipe := repo.Recipes["r1"]
	assert.Equal(t, "Soup", recipe.Title)
	assert.Equal(t, 2, recipe.Servings)
	assert.Empty(t, recipe.Ingredients) // nil/empty for missing fields
}

func TestHandleRecipeSubmitted_EmptyLLMData(t *testing.T) {
	svc, repo, llm, bus := newTestService()
	repo.Recipes["r1"] = &domain.Recipe{ID: "r1", Status: domain.StatusPending}

	// LLM returns valid JSON but all fields empty/zero
	llm.ProcessFunc = func(_ context.Context, _ string) (string, error) {
		return `{"title":"","ingredients":[],"instructions":[],"total_time":0,"servings":0,"course_type":""}`, nil
	}

	event := domain.RecipeSubmitted{RecipeID: "r1", RawInput: "test", Timestamp: time.Now()}
	err := svc.HandleRecipeSubmitted(context.Background(), event)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no structured data")

	// Status set to failed
	assert.Equal(t, domain.StatusFailed, repo.Recipes["r1"].Status)

	// Failure event published
	events := bus.Events()
	require.Len(t, events, 1)
	failed, ok := events[0].(domain.RecipeProcessingFailed)
	assert.True(t, ok)
	assert.Contains(t, failed.Error, "no structured data")
}
