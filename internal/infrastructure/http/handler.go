package http

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/bbroerse/recipe-processor/internal/domain"
	"github.com/google/uuid"
)

// RecipeService defines the application operations the handler depends on.
// Following the Go idiom: "accept interfaces, return structs" — the interface
// is owned by the consumer (infrastructure) rather than the producer (application).
type RecipeService interface {
	// SubmitRecipe validates and enqueues a raw recipe text for async LLM processing.
	SubmitRecipe(ctx context.Context, rawText string) (string, error)
	// GetRecipe retrieves a recipe by its unique ID.
	GetRecipe(ctx context.Context, id string) (*domain.Recipe, error)
}

// Handler handles HTTP requests for the recipe-processor API.
type Handler struct {
	service RecipeService
}

// NewHandler creates a new Handler with the given recipe service.
func NewHandler(service RecipeService) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes registers all API routes on the given ServeMux.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /recipes", h.submitRecipe)
	mux.HandleFunc("GET /recipes/{id}", h.getRecipe)
	mux.HandleFunc("GET /health", h.health)
}

// SubmitRequest is the request body for submitting a recipe.
type SubmitRequest struct {
	Text string `json:"text" example:"Mix flour, sugar, and eggs. Bake at 350°F for 30 minutes."`
}

// SubmitResponse is the response returned when a recipe is accepted for processing.
type SubmitResponse struct {
	ID     string `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Status string `json:"status" example:"pending"`
}

// ErrorResponse is the standard error response returned by the API.
type ErrorResponse struct {
	Error string `json:"error" example:"recipe text cannot be empty"`
	Code  string `json:"code" example:"EMPTY_TEXT"`
}

// HealthResponse is the response returned by the health check endpoint.
type HealthResponse struct {
	Status string `json:"status" example:"ok"`
	Time   string `json:"time" example:"2024-01-15T10:30:00Z"`
}

const maxRequestBodySize = 64 * 1024 // 64 KB

// submitRecipe creates a new recipe processing request.
//
//	@Summary		Submit a recipe for processing
//	@Description	Accepts raw recipe text and queues it for async LLM processing. Returns immediately with a recipe ID.
//	@Tags			recipes
//	@Accept			json
//	@Produce		json
//	@Param			request	body		SubmitRequest	true	"Recipe text to process"
//	@Success		202		{object}	SubmitResponse	"Recipe accepted for processing"
//	@Failure		400		{object}	ErrorResponse	"Invalid request (empty text, malformed JSON, or text too long)"
//	@Failure		413		{object}	ErrorResponse	"Request body exceeds 64KB limit"
//	@Failure		500		{object}	ErrorResponse	"Internal server error"
//	@Router			/recipes [post]
func (h *Handler) submitRecipe(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodySize)

	var req SubmitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		if err.Error() == "http: request body too large" {
			writeJSON(w, http.StatusRequestEntityTooLarge, ErrorResponse{Error: "request body too large", Code: "PAYLOAD_TOO_LARGE"})
			return
		}
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid request body", Code: "BAD_REQUEST"})
		return
	}

	id, err := h.service.SubmitRecipe(r.Context(), req.Text)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrEmptyRecipeText):
			writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: err.Error(), Code: "EMPTY_TEXT"})
		case errors.Is(err, domain.ErrTextTooLong):
			writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: err.Error(), Code: "TEXT_TOO_LONG"})
		default:
			slog.Error("failed to submit recipe", "error", err)
			writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "internal error", Code: "INTERNAL"})
		}
		return
	}

	writeJSON(w, http.StatusAccepted, SubmitResponse{ID: id, Status: string(domain.StatusPending)})
}

// getRecipe retrieves a recipe by its ID.
//
//	@Summary		Get a recipe by ID
//	@Description	Returns the full recipe object including structured data if LLM processing is complete.
//	@Tags			recipes
//	@Produce		json
//	@Param			id	path		string			true	"Recipe UUID"	format(uuid)
//	@Success		200	{object}	domain.Recipe	"Recipe found"
//	@Failure		400	{object}	ErrorResponse	"Invalid UUID format"
//	@Failure		404	{object}	ErrorResponse	"Recipe not found"
//	@Router			/recipes/{id} [get]
func (h *Handler) getRecipe(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, err := uuid.Parse(id); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid recipe id format", Code: "BAD_REQUEST"})
		return
	}

	recipe, err := h.service.GetRecipe(r.Context(), id)
	if err != nil {
		slog.Error("failed to get recipe", "id", id, "error", err) // #nosec G706 -- structured logging, no injection risk
		writeJSON(w, http.StatusNotFound, ErrorResponse{Error: "recipe not found", Code: "NOT_FOUND"})
		return
	}

	writeJSON(w, http.StatusOK, recipe)
}

// health returns the API health status.
//
//	@Summary		Health check
//	@Description	Returns current API health status and server time.
//	@Tags			system
//	@Produce		json
//	@Success		200	{object}	HealthResponse	"Health status with server time"
//	@Router			/health [get]
func (h *Handler) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, HealthResponse{
		Status: "ok",
		Time:   time.Now().UTC().Format(time.RFC3339),
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
