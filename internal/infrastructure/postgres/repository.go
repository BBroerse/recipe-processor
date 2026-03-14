package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bbroerse/recipe-processor/internal/domain"
)

const dbTimeout = 5 * time.Second

type RecipeRepository struct {
	pool *pgxpool.Pool
}

func NewRecipeRepository(pool *pgxpool.Pool) *RecipeRepository {
	return &RecipeRepository{pool: pool}
}

func (r *RecipeRepository) Save(ctx context.Context, recipe *domain.Recipe) error {
	ctx, cancel := context.WithTimeout(ctx, dbTimeout)
	defer cancel()

	_, err := r.pool.Exec(ctx,
		`INSERT INTO recipes (id, raw_input, raw_response, title, ingredients, instructions,
		 total_time, servings, course_type, status, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`,
		recipe.ID, recipe.RawInput, recipe.RawResponse,
		recipe.Title, recipe.Ingredients, recipe.Instructions,
		recipe.TotalTime, recipe.Servings, recipe.CourseType,
		recipe.Status, recipe.CreatedAt, recipe.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("saving recipe: %w", err)
	}
	return nil
}

func (r *RecipeRepository) FindByID(ctx context.Context, id string) (*domain.Recipe, error) {
	ctx, cancel := context.WithTimeout(ctx, dbTimeout)
	defer cancel()

	var recipe domain.Recipe
	err := r.pool.QueryRow(ctx,
		`SELECT id, raw_input, raw_response, title, ingredients, instructions,
		 total_time, servings, course_type, status, created_at, updated_at
		 FROM recipes WHERE id = $1`, id,
	).Scan(
		&recipe.ID, &recipe.RawInput, &recipe.RawResponse,
		&recipe.Title, &recipe.Ingredients, &recipe.Instructions,
		&recipe.TotalTime, &recipe.Servings, &recipe.CourseType,
		&recipe.Status, &recipe.CreatedAt, &recipe.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("finding recipe %s: %w", id, err)
	}
	return &recipe, nil
}

func (r *RecipeRepository) UpdateStatus(ctx context.Context, id string, status domain.RecipeStatus) error {
	ctx, cancel := context.WithTimeout(ctx, dbTimeout)
	defer cancel()

	_, err := r.pool.Exec(ctx,
		`UPDATE recipes SET status = $1, updated_at = $2 WHERE id = $3`,
		status, time.Now().UTC(), id,
	)
	if err != nil {
		return fmt.Errorf("updating recipe status: %w", err)
	}
	return nil
}

// UpdateResult saves the structured LLM output and raw response.
func (r *RecipeRepository) UpdateResult(ctx context.Context, recipe *domain.Recipe) error {
	ctx, cancel := context.WithTimeout(ctx, dbTimeout)
	defer cancel()

	_, err := r.pool.Exec(ctx,
		`UPDATE recipes SET raw_response = $1, title = $2, ingredients = $3, instructions = $4,
		 total_time = $5, servings = $6, course_type = $7, status = $8, updated_at = $9
		 WHERE id = $10`,
		recipe.RawResponse, recipe.Title, recipe.Ingredients, recipe.Instructions,
		recipe.TotalTime, recipe.Servings, recipe.CourseType,
		domain.StatusCompleted, time.Now().UTC(), recipe.ID,
	)
	if err != nil {
		return fmt.Errorf("updating recipe result: %w", err)
	}
	return nil
}
