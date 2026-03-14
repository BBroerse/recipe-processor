package e2e_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bbroerse/recipe-processor/internal/application"
	"github.com/bbroerse/recipe-processor/internal/domain"
	handler "github.com/bbroerse/recipe-processor/internal/infrastructure/http"
	"github.com/bbroerse/recipe-processor/internal/shared/eventbus"
	"github.com/bbroerse/recipe-processor/internal/testutil"
)

// TestE2E_SubmitAndProcessRecipe tests the full flow:
// HTTP POST → save to repo → publish event → LLM processes → structured data saved
func TestE2E_SubmitAndProcessRecipe(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test in short mode")
	}

	repo := testutil.NewMockRecipeRepository()
	eventLog := testutil.NewMockEventLogRepository()

	llmResponse := `{
		"title": "Classic Pancakes",
		"ingredients": ["2 cups flour", "2 eggs", "1 cup milk", "2 tbsp sugar"],
		"instructions": ["Mix dry ingredients", "Add wet ingredients", "Cook on griddle"],
		"total_time": 20,
		"servings": 4,
		"course_type": "main"
	}`
	llm := &testutil.MockLLMProvider{
		ProcessFunc: func(_ context.Context, _ string) (string, error) {
			return llmResponse, nil
		},
	}

	bus := eventbus.New(100, eventLog)
	svc := application.NewRecipeService(repo, llm, bus)

	// Wire event handler
	bus.Subscribe("recipe.submitted", svc.HandleRecipeSubmitted)

	// Start event bus
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	require.NoError(t, bus.Start(ctx))

	// Set up HTTP server
	mux := http.NewServeMux()
	h := handler.NewHandler(svc)
	h.RegisterRoutes(mux)
	srv := httptest.NewServer(handler.LoggingMiddleware(mux))
	defer srv.Close()

	// 1. Submit recipe via HTTP
	payload := `{"text":"Make me some classic pancakes with flour eggs and milk"}`
	resp, err := http.Post(srv.URL+"/recipes", "application/json", bytes.NewBufferString(payload))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusAccepted, resp.StatusCode)

	var submitResp map[string]string
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&submitResp))
	recipeID := submitResp["id"]
	assert.NotEmpty(t, recipeID)
	assert.Equal(t, "pending", submitResp["status"])

	// 2. Wait for async processing to complete
	assert.Eventually(t, func() bool {
		r, findErr := repo.FindByID(context.Background(), recipeID)
		return findErr == nil && r.Status == domain.StatusCompleted
	}, 5*time.Second, 50*time.Millisecond, "recipe should be processed within timeout")

	// 3. Retrieve processed recipe via HTTP
	resp2, err := http.Get(fmt.Sprintf("%s/recipes/%s", srv.URL, recipeID))
	require.NoError(t, err)
	defer resp2.Body.Close()

	assert.Equal(t, http.StatusOK, resp2.StatusCode)

	var recipe domain.Recipe
	require.NoError(t, json.NewDecoder(resp2.Body).Decode(&recipe))

	// 4. Verify structured data
	assert.Equal(t, "Classic Pancakes", recipe.Title)
	assert.Equal(t, []string{"2 cups flour", "2 eggs", "1 cup milk", "2 tbsp sugar"}, recipe.Ingredients)
	assert.Equal(t, []string{"Mix dry ingredients", "Add wet ingredients", "Cook on griddle"}, recipe.Instructions)
	assert.Equal(t, 20, recipe.TotalTime)
	assert.Equal(t, 4, recipe.Servings)
	assert.Equal(t, "main", recipe.CourseType)
	assert.Equal(t, domain.StatusCompleted, recipe.Status)
	assert.Contains(t, recipe.RawInput, "pancakes", "raw input should be preserved")

	// 5. Verify events were logged
	entries := eventLog.GetEntries()
	assert.GreaterOrEqual(t, len(entries), 2, "should have at least RecipeSubmitted + RecipeProcessed events")

	eventTypes := make([]string, len(entries))
	for i, e := range entries {
		eventTypes[i] = e.EventType
	}
	assert.Contains(t, eventTypes, "recipe.submitted")
	assert.Contains(t, eventTypes, "recipe.processed")
}

// TestE2E_SubmitAndLLMFails tests the failure path:
// HTTP POST → save → publish → LLM fails → status set to failed
func TestE2E_SubmitAndLLMFails(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test in short mode")
	}

	repo := testutil.NewMockRecipeRepository()
	eventLog := testutil.NewMockEventLogRepository()

	llm := &testutil.MockLLMProvider{
		ProcessFunc: func(_ context.Context, _ string) (string, error) {
			return "", fmt.Errorf("model not loaded")
		},
	}

	bus := eventbus.New(100, eventLog)
	svc := application.NewRecipeService(repo, llm, bus)
	bus.Subscribe("recipe.submitted", svc.HandleRecipeSubmitted)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	require.NoError(t, bus.Start(ctx))

	mux := http.NewServeMux()
	h := handler.NewHandler(svc)
	h.RegisterRoutes(mux)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	// Submit recipe
	resp, err := http.Post(srv.URL+"/recipes", "application/json", bytes.NewBufferString(`{"text":"test"}`))
	require.NoError(t, err)
	defer resp.Body.Close()

	var submitResp map[string]string
	json.NewDecoder(resp.Body).Decode(&submitResp)
	recipeID := submitResp["id"]

	// Wait for failure processing
	assert.Eventually(t, func() bool {
		r, err := repo.FindByID(context.Background(), recipeID)
		return err == nil && r.Status == domain.StatusFailed
	}, 5*time.Second, 50*time.Millisecond)

	// Verify failure event logged
	entries := eventLog.GetEntries()
	eventTypes := make([]string, len(entries))
	for i, e := range entries {
		eventTypes[i] = e.EventType
	}
	assert.Contains(t, eventTypes, "recipe.processing_failed")
}

// TestE2E_HealthCheck verifies the health endpoint works in a full setup
func TestE2E_HealthCheck(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test in short mode")
	}

	repo := testutil.NewMockRecipeRepository()
	eventLog := testutil.NewMockEventLogRepository()
	llm := &testutil.MockLLMProvider{ProcessFunc: func(_ context.Context, _ string) (string, error) { return "{}", nil }}
	bus := eventbus.New(100, eventLog)
	svc := application.NewRecipeService(repo, llm, bus)

	mux := http.NewServeMux()
	h := handler.NewHandler(svc)
	h.RegisterRoutes(mux)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/health")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}
