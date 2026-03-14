package domain

import "context"

type RecipeRepository interface {
	Save(ctx context.Context, recipe *Recipe) error
	FindByID(ctx context.Context, id string) (*Recipe, error)
	UpdateStatus(ctx context.Context, id string, status RecipeStatus) error
	UpdateResult(ctx context.Context, recipe *Recipe) error
}

// EventLogRepository persists every domain event for debugging and audit.
type EventLogRepository interface {
	Log(ctx context.Context, entry *EventLogEntry) error
}

type LLMProvider interface {
	Process(ctx context.Context, input string) (string, error)
}

type EventBus interface {
	Publish(ctx context.Context, event Event) error
	Subscribe(eventType string, handler EventHandler)
	Start(ctx context.Context) error
	Stop() error
}

type EventHandler func(ctx context.Context, event Event) error
