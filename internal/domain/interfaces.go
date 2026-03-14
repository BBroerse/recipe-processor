package domain

import "context"

// RecipeRepository defines persistence operations for recipes.
type RecipeRepository interface {
	Save(ctx context.Context, recipe *Recipe) error
	FindByID(ctx context.Context, id string) (*Recipe, error)
	UpdateStatus(ctx context.Context, id string, status RecipeStatus) error
	UpdateResult(ctx context.Context, recipe *Recipe) error
}

// EventLogRepository persists domain events for debugging and audit.
type EventLogRepository interface {
	Log(ctx context.Context, entry *EventLogEntry) error
}

// LLMProvider processes raw text through a language model and returns the raw response.
type LLMProvider interface {
	Process(ctx context.Context, input string) (string, error)
}

// EventBus provides async publish/subscribe for domain events.
type EventBus interface {
	Publish(ctx context.Context, event Event) error
	Subscribe(eventType string, handler EventHandler)
	Start(ctx context.Context) error
	Stop() error
}

// EventHandler is a function that handles a domain event.
type EventHandler func(ctx context.Context, event Event) error
