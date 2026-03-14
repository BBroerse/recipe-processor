package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bbroerse/recipe-processor/internal/application"
	"github.com/bbroerse/recipe-processor/internal/domain"
	handler "github.com/bbroerse/recipe-processor/internal/infrastructure/http"
	"github.com/bbroerse/recipe-processor/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestServer() (*httptest.Server, *testutil.MockRecipeRepository) {
	repo := testutil.NewMockRecipeRepository()
	llm := &testutil.MockLLMProvider{
		ProcessFunc: func(_ context.Context, _ string) (string, error) {
			return "{}", nil
		},
	}
	bus := testutil.NewMockEventBus()
	svc := application.NewRecipeService(repo, llm, bus)
	h := handler.NewHandler(svc)

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)
	return httptest.NewServer(mux), repo
}

func TestHealth(t *testing.T) {
	srv, _ := setupTestServer()
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/health")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body map[string]string
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, "ok", body["status"])
	assert.NotEmpty(t, body["time"])
}

func TestSubmitRecipe_HTTP_Success(t *testing.T) {
	srv, _ := setupTestServer()
	defer srv.Close()

	payload := `{"text":"Mix flour with eggs and milk"}`
	resp, err := http.Post(srv.URL+"/recipes", "application/json", bytes.NewBufferString(payload))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusAccepted, resp.StatusCode)

	var body map[string]string
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.NotEmpty(t, body["id"])
	assert.Equal(t, "pending", body["status"])
}

func TestSubmitRecipe_HTTP_EmptyText(t *testing.T) {
	srv, _ := setupTestServer()
	defer srv.Close()

	payload := `{"text":""}`
	resp, err := http.Post(srv.URL+"/recipes", "application/json", bytes.NewBufferString(payload))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	var body map[string]string
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, "EMPTY_TEXT", body["code"])
}

func TestSubmitRecipe_HTTP_InvalidJSON(t *testing.T) {
	srv, _ := setupTestServer()
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/recipes", "application/json", bytes.NewBufferString("not json"))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	var body map[string]string
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, "BAD_REQUEST", body["code"])
}

func TestGetRecipe_HTTP_Success(t *testing.T) {
	srv, repo := setupTestServer()
	defer srv.Close()

	testID := "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
	repo.Recipes[testID] = &domain.Recipe{
		ID:          testID,
		RawInput:    "Some recipe",
		Title:       "Pasta",
		Ingredients: []string{"pasta", "sauce"},
		Status:      domain.StatusCompleted,
	}

	resp, err := http.Get(srv.URL + "/recipes/" + testID)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

	var recipe domain.Recipe
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&recipe))
	assert.Equal(t, "Pasta", recipe.Title)
	assert.Equal(t, []string{"pasta", "sauce"}, recipe.Ingredients)
}

func TestGetRecipe_HTTP_NotFound(t *testing.T) {
	srv, _ := setupTestServer()
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/recipes/00000000-0000-0000-0000-000000000000")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestSubmitRecipe_HTTP_ContentType(t *testing.T) {
	srv, _ := setupTestServer()
	defer srv.Close()

	payload := `{"text":"Test recipe"}`
	resp, err := http.Post(srv.URL+"/recipes", "application/json", bytes.NewBufferString(payload))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))
}
