package testutil

import (
	"context"
	"sync"

	"github.com/bbroerse/recipe-processor/internal/domain"
)

// MockRecipeRepository is a test double for domain.RecipeRepository.
type MockRecipeRepository struct {
	mu      sync.Mutex
	Recipes map[string]*domain.Recipe

	SaveFunc         func(ctx context.Context, recipe *domain.Recipe) error
	FindByIDFunc     func(ctx context.Context, id string) (*domain.Recipe, error)
	UpdateStatusFunc func(ctx context.Context, id string, status domain.RecipeStatus) error
	UpdateResultFunc func(ctx context.Context, recipe *domain.Recipe) error
}

// NewMockRecipeRepository creates a MockRecipeRepository with default in-memory implementations.
func NewMockRecipeRepository() *MockRecipeRepository {
	m := &MockRecipeRepository{
		Recipes: make(map[string]*domain.Recipe),
	}
	m.SaveFunc = func(_ context.Context, recipe *domain.Recipe) error {
		m.mu.Lock()
		defer m.mu.Unlock()
		m.Recipes[recipe.ID] = recipe
		return nil
	}
	m.FindByIDFunc = func(_ context.Context, id string) (*domain.Recipe, error) {
		m.mu.Lock()
		defer m.mu.Unlock()
		r, ok := m.Recipes[id]
		if !ok {
			return nil, domain.ErrRecipeNotFound
		}
		return r, nil
	}
	m.UpdateStatusFunc = func(_ context.Context, id string, status domain.RecipeStatus) error {
		m.mu.Lock()
		defer m.mu.Unlock()
		if r, ok := m.Recipes[id]; ok {
			r.Status = status
		}
		return nil
	}
	m.UpdateResultFunc = func(_ context.Context, recipe *domain.Recipe) error {
		m.mu.Lock()
		defer m.mu.Unlock()
		if r, ok := m.Recipes[recipe.ID]; ok {
			r.RawResponse = recipe.RawResponse
			r.Title = recipe.Title
			r.Ingredients = recipe.Ingredients
			r.Instructions = recipe.Instructions
			r.TotalTime = recipe.TotalTime
			r.Servings = recipe.Servings
			r.CourseType = recipe.CourseType
			r.Status = domain.StatusCompleted
		}
		return nil
	}
	return m
}

// Save delegates to SaveFunc.
func (m *MockRecipeRepository) Save(ctx context.Context, recipe *domain.Recipe) error {
	return m.SaveFunc(ctx, recipe)
}
// FindByID delegates to FindByIDFunc.
func (m *MockRecipeRepository) FindByID(ctx context.Context, id string) (*domain.Recipe, error) {
	return m.FindByIDFunc(ctx, id)
}
// UpdateStatus delegates to UpdateStatusFunc.
func (m *MockRecipeRepository) UpdateStatus(ctx context.Context, id string, status domain.RecipeStatus) error {
	return m.UpdateStatusFunc(ctx, id, status)
}
// UpdateResult delegates to UpdateResultFunc.
func (m *MockRecipeRepository) UpdateResult(ctx context.Context, recipe *domain.Recipe) error {
	return m.UpdateResultFunc(ctx, recipe)
}

// MockLLMProvider is a test double for domain.LLMProvider.
type MockLLMProvider struct {
	ProcessFunc func(ctx context.Context, input string) (string, error)
}

// Process delegates to ProcessFunc.
func (m *MockLLMProvider) Process(ctx context.Context, input string) (string, error) {
	return m.ProcessFunc(ctx, input)
}

// MockEventBus is a test double for domain.EventBus.
type MockEventBus struct {
	mu              sync.Mutex
	PublishedEvents []domain.Event
	Handlers        map[string][]domain.EventHandler

	PublishFunc func(ctx context.Context, event domain.Event) error
}

// NewMockEventBus creates a MockEventBus with default publish behavior.
func NewMockEventBus() *MockEventBus {
	m := &MockEventBus{
		Handlers: make(map[string][]domain.EventHandler),
	}
	m.PublishFunc = func(_ context.Context, event domain.Event) error {
		m.mu.Lock()
		defer m.mu.Unlock()
		m.PublishedEvents = append(m.PublishedEvents, event)
		return nil
	}
	return m
}

// Publish delegates to PublishFunc.
func (m *MockEventBus) Publish(ctx context.Context, event domain.Event) error {
	return m.PublishFunc(ctx, event)
}
// Subscribe registers a handler for a specific event type.
func (m *MockEventBus) Subscribe(eventType string, handler domain.EventHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Handlers[eventType] = append(m.Handlers[eventType], handler)
}
// Start is a no-op for the mock.
func (m *MockEventBus) Start(_ context.Context) error { return nil }

// Stop is a no-op for the mock.
func (m *MockEventBus) Stop() error { return nil }

// Events returns a copy of all published events.
func (m *MockEventBus) Events() []domain.Event {
	m.mu.Lock()
	defer m.mu.Unlock()
	copied := make([]domain.Event, len(m.PublishedEvents))
	copy(copied, m.PublishedEvents)
	return copied
}

// MockEventLogRepository is a test double for domain.EventLogRepository.
type MockEventLogRepository struct {
	mu      sync.Mutex
	Entries []domain.EventLogEntry

	LogFunc func(ctx context.Context, entry *domain.EventLogEntry) error
}

// NewMockEventLogRepository creates a MockEventLogRepository with default log behavior.
func NewMockEventLogRepository() *MockEventLogRepository {
	m := &MockEventLogRepository{}
	m.LogFunc = func(_ context.Context, entry *domain.EventLogEntry) error {
		m.mu.Lock()
		defer m.mu.Unlock()
		m.Entries = append(m.Entries, *entry)
		return nil
	}
	return m
}

// Log delegates to LogFunc.
func (m *MockEventLogRepository) Log(ctx context.Context, entry *domain.EventLogEntry) error {
	return m.LogFunc(ctx, entry)
}

// GetEntries returns a copy of all logged event entries.
func (m *MockEventLogRepository) GetEntries() []domain.EventLogEntry {
	m.mu.Lock()
	defer m.mu.Unlock()
	copied := make([]domain.EventLogEntry, len(m.Entries))
	copy(copied, m.Entries)
	return copied
}
