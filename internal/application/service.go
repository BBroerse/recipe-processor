package application

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/bbroerse/recipe-processor/internal/domain"
	"github.com/google/uuid"
)

type RecipeService struct {
	repo     domain.RecipeRepository
	llm      domain.LLMProvider
	eventBus domain.EventBus
}

func NewRecipeService(repo domain.RecipeRepository, llm domain.LLMProvider, eventBus domain.EventBus) *RecipeService {
	return &RecipeService{
		repo:     repo,
		llm:      llm,
		eventBus: eventBus,
	}
}

// SubmitRecipe validates input, saves a pending recipe, and publishes an event for async processing.
func (s *RecipeService) SubmitRecipe(ctx context.Context, rawText string) (string, error) {
	if rawText == "" {
		return "", domain.ErrEmptyRecipeText
	}
	if len(rawText) > 10_000 {
		return "", domain.ErrTextTooLong
	}

	id := uuid.New().String()
	now := time.Now().UTC()

	recipe := &domain.Recipe{
		ID:           id,
		RawInput:     rawText,
		Status:       domain.StatusPending,
		CreatedAt:    now,
		UpdatedAt:    now,
		// Initialize structured fields to empty values to satisfy NOT NULL constraints
		Title:        "",
		Ingredients:  []string{},
		Instructions: []string{},
		TotalTime:    0,
		Servings:     0,
		CourseType:   "",
	}

	if err := s.repo.Save(ctx, recipe); err != nil {
		return "", fmt.Errorf("saving recipe: %w", err)
	}

	event := domain.RecipeSubmitted{
		RecipeID:  id,
		RawInput:  rawText,
		Timestamp: now,
	}
	if err := s.eventBus.Publish(ctx, event); err != nil {
		slog.Error("failed to publish recipe submitted event", "recipe_id", id, "error", err)
	}

	return id, nil
}

// GetRecipe retrieves a recipe by ID.
func (s *RecipeService) GetRecipe(ctx context.Context, id string) (*domain.Recipe, error) {
	recipe, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("finding recipe: %w", err)
	}
	return recipe, nil
}

// parsedRecipe mirrors the JSON structure the LLM returns.
type parsedRecipe struct {
	Title        string   `json:"title"`
	Ingredients  []string `json:"ingredients"`
	Instructions []string `json:"instructions"`
	TotalTime    int      `json:"total_time"`
	Servings     int      `json:"servings"`
	CourseType   string   `json:"course_type"`
}

// HandleRecipeSubmitted processes a submitted recipe through the LLM.
func (s *RecipeService) HandleRecipeSubmitted(ctx context.Context, event domain.Event) error {
	submitted, ok := event.(domain.RecipeSubmitted)
	if !ok {
		return fmt.Errorf("unexpected event type: %T", event)
	}

	slog.Info("processing recipe", "recipe_id", submitted.RecipeID)

	if err := s.repo.UpdateStatus(ctx, submitted.RecipeID, domain.StatusProcessing); err != nil {
		return fmt.Errorf("updating status to processing: %w", err)
	}

	response, err := s.llm.Process(ctx, submitted.RawInput)
	if err != nil {
		slog.Error("llm processing failed", "recipe_id", submitted.RecipeID, "error", err)
		_ = s.repo.UpdateStatus(ctx, submitted.RecipeID, domain.StatusFailed)

		_ = s.eventBus.Publish(ctx, domain.RecipeProcessingFailed{
			RecipeID:  submitted.RecipeID,
			Error:     err.Error(),
			Timestamp: time.Now().UTC(),
		})
		return fmt.Errorf("llm processing: %w", err)
	}

	// Parse LLM response into structured fields
	var parsed parsedRecipe
	if err := json.Unmarshal([]byte(response), &parsed); err != nil {
		slog.Warn("failed to parse LLM response as JSON, storing raw only",
			"recipe_id", submitted.RecipeID, "error", err)
		// Mark as completed with empty structured data rather than failing
		recipe := &domain.Recipe{
			ID:           submitted.RecipeID,
			RawResponse:  response,
			Title:        "",
			Ingredients:  []string{},
			Instructions: []string{},
			TotalTime:    0,
			Servings:     0,
			CourseType:   "",
		}
		if err := s.repo.UpdateResult(ctx, recipe); err != nil {
			return fmt.Errorf("saving recipe result: %w", err)
		}
		return nil
	}

	// Validate that we got at least some meaningful data
	if parsed.Title == "" && len(parsed.Ingredients) == 0 && len(parsed.Instructions) == 0 {
		slog.Warn("LLM returned empty structured data",
			"recipe_id", submitted.RecipeID, "raw_response", response)
		_ = s.repo.UpdateStatus(ctx, submitted.RecipeID, domain.StatusFailed)
		_ = s.eventBus.Publish(ctx, domain.RecipeProcessingFailed{
			RecipeID:  submitted.RecipeID,
			Error:     "LLM returned no structured data",
			Timestamp: time.Now().UTC(),
		})
		return fmt.Errorf("LLM returned no structured data")
	}

	// Ensure arrays are never nil (convert nil to empty slice)
	if parsed.Ingredients == nil {
		parsed.Ingredients = []string{}
	}
	if parsed.Instructions == nil {
		parsed.Instructions = []string{}
	}

	recipe := &domain.Recipe{
		ID:           submitted.RecipeID,
		RawResponse:  response,
		Title:        parsed.Title,
		Ingredients:  parsed.Ingredients,
		Instructions: parsed.Instructions,
		TotalTime:    parsed.TotalTime,
		Servings:     parsed.Servings,
		CourseType:   parsed.CourseType,
	}

	if err := s.repo.UpdateResult(ctx, recipe); err != nil {
		return fmt.Errorf("saving recipe result: %w", err)
	}

	_ = s.eventBus.Publish(ctx, domain.RecipeProcessed{
		RecipeID:    submitted.RecipeID,
		RawResponse: response,
		Timestamp:   time.Now().UTC(),
	})

	slog.Info("recipe processed", "recipe_id", submitted.RecipeID, "title", parsed.Title)
	return nil
}
