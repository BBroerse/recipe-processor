package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/bbroerse/recipe-processor/internal/domain"
	"github.com/bbroerse/recipe-processor/internal/infrastructure/postgres"
)

func setupTestDB(t *testing.T) (*pgxpool.Pool, func()) {
	t.Helper()
	ctx := context.Background()

	container, err := tcpostgres.Run(ctx, "postgres:16-alpine",
		tcpostgres.WithDatabase("testdb"),
		tcpostgres.WithUsername("test"),
		tcpostgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	require.NoError(t, err)

	connStr, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	// Run embedded migrations (no filesystem path needed).
	require.NoError(t, postgres.Migrate(connStr))

	pool, err := pgxpool.New(ctx, connStr)
	require.NoError(t, err)

	return pool, func() {
		pool.Close()
		container.Terminate(ctx)
	}
}

func TestRecipeRepository_SaveAndFindByID(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	pool, cleanup := setupTestDB(t)
	defer cleanup()

	repo := postgres.NewRecipeRepository(pool)
	ctx := context.Background()

	recipe := &domain.Recipe{
		ID:           "test-1",
		RawInput:     "Mix flour and eggs",
		Status:       domain.StatusPending,
		Title:        "",
		Ingredients:  []string{},
		Instructions: []string{},
		CreatedAt:    time.Now().UTC().Truncate(time.Microsecond),
		UpdatedAt:    time.Now().UTC().Truncate(time.Microsecond),
	}

	err := repo.Save(ctx, recipe)
	require.NoError(t, err)

	found, err := repo.FindByID(ctx, "test-1")
	require.NoError(t, err)
	assert.Equal(t, "test-1", found.ID)
	assert.Equal(t, "Mix flour and eggs", found.RawInput)
	assert.Equal(t, domain.StatusPending, found.Status)
}

func TestRecipeRepository_UpdateStatus(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	pool, cleanup := setupTestDB(t)
	defer cleanup()

	repo := postgres.NewRecipeRepository(pool)
	ctx := context.Background()

	recipe := &domain.Recipe{
		ID:           "test-2",
		RawInput:     "test",
		Status:       domain.StatusPending,
		Ingredients:  []string{},
		Instructions: []string{},
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	require.NoError(t, repo.Save(ctx, recipe))

	require.NoError(t, repo.UpdateStatus(ctx, "test-2", domain.StatusProcessing))

	found, err := repo.FindByID(ctx, "test-2")
	require.NoError(t, err)
	assert.Equal(t, domain.StatusProcessing, found.Status)
}

func TestRecipeRepository_UpdateResult(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	pool, cleanup := setupTestDB(t)
	defer cleanup()

	repo := postgres.NewRecipeRepository(pool)
	ctx := context.Background()

	recipe := &domain.Recipe{
		ID:           "test-3",
		RawInput:     "test",
		Status:       domain.StatusProcessing,
		Ingredients:  []string{},
		Instructions: []string{},
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	require.NoError(t, repo.Save(ctx, recipe))

	update := &domain.Recipe{
		ID:           "test-3",
		RawResponse:  "{\"title\":\"Pasta\"}",
		Title:        "Pasta",
		Ingredients:  []string{"pasta", "tomato", "cheese"},
		Instructions: []string{"boil", "mix", "serve"},
		TotalTime:    25,
		Servings:     4,
		CourseType:   "main",
	}
	require.NoError(t, repo.UpdateResult(ctx, update))

	found, err := repo.FindByID(ctx, "test-3")
	require.NoError(t, err)
	assert.Equal(t, domain.StatusCompleted, found.Status)
	assert.Equal(t, "Pasta", found.Title)
	assert.Equal(t, []string{"pasta", "tomato", "cheese"}, found.Ingredients)
	assert.Equal(t, []string{"boil", "mix", "serve"}, found.Instructions)
	assert.Equal(t, 25, found.TotalTime)
	assert.Equal(t, 4, found.Servings)
	assert.Equal(t, "main", found.CourseType)
	assert.Equal(t, "{\"title\":\"Pasta\"}", found.RawResponse)
}

func TestRecipeRepository_FindByID_NotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	pool, cleanup := setupTestDB(t)
	defer cleanup()

	repo := postgres.NewRecipeRepository(pool)
	_, err := repo.FindByID(context.Background(), "nonexistent")
	assert.Error(t, err)
}

func TestEventLogRepository_Log(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	pool, cleanup := setupTestDB(t)
	defer cleanup()

	repo := postgres.NewEventLogRepository(pool)
	ctx := context.Background()

	entry := domain.EventLogEntry{
		ID:        "evt-1",
		EventType: "recipe.submitted",
		RecipeID:  "r-1",
		Payload:   "{\"recipe_id\":\"r-1\",\"raw_input\":\"test\"}",
		CreatedAt: time.Now().UTC(),
	}

	err := repo.Log(ctx, &entry)
	require.NoError(t, err)

	// Verify by querying directly
	var count int
	err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM event_log WHERE recipe_id = $1", "r-1").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}
