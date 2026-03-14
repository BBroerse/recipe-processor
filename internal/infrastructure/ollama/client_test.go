package ollama_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bbroerse/recipe-processor/internal/infrastructure/ollama"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOllamaClient_Process_Success(t *testing.T) {
	expectedResponse := `{"title":"Pancakes","ingredients":["flour"],"instructions":["mix"],"total_time":30,"servings":4,"course_type":"main"}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/generate", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)

		var req map[string]interface{}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		assert.Equal(t, "test-model", req["model"])
		assert.Equal(t, false, req["stream"])
		assert.NotEmpty(t, req["system"])
		assert.Equal(t, "Make pancakes", req["prompt"])

		json.NewEncoder(w).Encode(map[string]string{"response": expectedResponse})
	}))
	defer server.Close()

	client := ollama.NewClient(server.URL, "test-model")
	result, err := client.Process(context.Background(), "Make pancakes")

	require.NoError(t, err)
	assert.Equal(t, expectedResponse, result)
}

func TestOllamaClient_Process_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	}))
	defer server.Close()

	client := ollama.NewClient(server.URL, "test-model")
	_, err := client.Process(context.Background(), "test")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "status 500")
}

func TestOllamaClient_Process_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("not json"))
	}))
	defer server.Close()

	client := ollama.NewClient(server.URL, "test-model")
	_, err := client.Process(context.Background(), "test")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "decoding")
}

func TestOllamaClient_Process_ContextCancelled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// Slow response
		select {}
	}))
	defer server.Close()

	client := ollama.NewClient(server.URL, "test-model")
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := client.Process(ctx, "test")
	assert.Error(t, err)
}

func TestOllamaClient_SystemPrompt(t *testing.T) {
	var receivedSystem string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]interface{}
		json.NewDecoder(r.Body).Decode(&req)
		receivedSystem = req["system"].(string)
		json.NewEncoder(w).Encode(map[string]string{"response": "{}"})
	}))
	defer server.Close()

	client := ollama.NewClient(server.URL, "test-model")
	_, _ = client.Process(context.Background(), "test")

	assert.Contains(t, receivedSystem, "recipe parser")
	assert.Contains(t, receivedSystem, "JSON")
}
