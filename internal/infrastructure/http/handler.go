package http

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/bbroerse/recipe-processor/internal/application"
	"github.com/bbroerse/recipe-processor/internal/domain"
)

type Handler struct {
	service *application.RecipeService
}

func NewHandler(service *application.RecipeService) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /recipes", h.submitRecipe)
	mux.HandleFunc("GET /recipes/{id}", h.getRecipe)
	mux.HandleFunc("GET /health", h.health)
}

type submitRequest struct {
	Text string `json:"text"`
}

type submitResponse struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

type errorResponse struct {
	Error string `json:"error"`
	Code  string `json:"code"`
}

func (h *Handler) submitRecipe(w http.ResponseWriter, r *http.Request) {
	var req submitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid request body", Code: "BAD_REQUEST"})
		return
	}

	id, err := h.service.SubmitRecipe(r.Context(), req.Text)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrEmptyRecipeText):
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error(), Code: "EMPTY_TEXT"})
		case errors.Is(err, domain.ErrTextTooLong):
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error(), Code: "TEXT_TOO_LONG"})
		default:
			slog.Error("failed to submit recipe", "error", err)
			writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "internal error", Code: "INTERNAL"})
		}
		return
	}

	writeJSON(w, http.StatusAccepted, submitResponse{ID: id, Status: string(domain.StatusPending)})
}

func (h *Handler) getRecipe(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "missing recipe id", Code: "BAD_REQUEST"})
		return
	}

	recipe, err := h.service.GetRecipe(r.Context(), id)
	if err != nil {
		slog.Error("failed to get recipe", "id", id, "error", err)
		writeJSON(w, http.StatusNotFound, errorResponse{Error: "recipe not found", Code: "NOT_FOUND"})
		return
	}

	writeJSON(w, http.StatusOK, recipe)
}

func (h *Handler) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "time": time.Now().UTC().Format(time.RFC3339)})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
