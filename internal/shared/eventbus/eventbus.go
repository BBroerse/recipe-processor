package eventbus

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/bbroerse/recipe-processor/internal/domain"
	"github.com/google/uuid"
)

// InMemoryEventBus is a channel-based event bus that dispatches events to registered handlers.
type InMemoryEventBus struct {
	handlers map[string][]domain.EventHandler
	eventLog domain.EventLogRepository
	ch       chan domain.Event
	wg       sync.WaitGroup
	mu       sync.RWMutex
}

// New creates an InMemoryEventBus with the given buffer size and event log repository.
func New(bufferSize int, eventLog domain.EventLogRepository) *InMemoryEventBus {
	return &InMemoryEventBus{
		handlers: make(map[string][]domain.EventHandler),
		eventLog: eventLog,
		ch:       make(chan domain.Event, bufferSize),
	}
}

// Publish enqueues an event onto the bus channel.
func (b *InMemoryEventBus) Publish(_ context.Context, event domain.Event) error {
	select {
	case b.ch <- event:
		return nil
	default:
		return fmt.Errorf("event bus buffer full, dropping event: %s", event.EventType())
	}
}

// Subscribe registers a handler for a specific event type.
func (b *InMemoryEventBus) Subscribe(eventType string, handler domain.EventHandler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers[eventType] = append(b.handlers[eventType], handler)
}

// Start begins the event processing goroutine.
func (b *InMemoryEventBus) Start(ctx context.Context) error {
	b.wg.Add(1)
	go func() {
		defer b.wg.Done()
		for {
			select {
			case event := <-b.ch:
				b.persistEvent(ctx, event)
				b.dispatch(ctx, event)
			case <-ctx.Done():
				// Drain remaining events with a fresh context since the parent is cancelled
				drainCtx := context.Background() //nolint:contextcheck // intentional: parent ctx is cancelled // #nosec G118
				for {
					select {
					case event := <-b.ch:
						b.persistEvent(drainCtx, event)
						b.dispatch(drainCtx, event)
					default:
						return
					}
				}
			}
		}
	}()
	return nil
}

// Stop waits for all in-flight events to be processed.
func (b *InMemoryEventBus) Stop() error {
	b.wg.Wait()
	return nil
}

func (b *InMemoryEventBus) persistEvent(ctx context.Context, event domain.Event) {
	payload, err := json.Marshal(event)
	if err != nil {
		slog.Error("failed to marshal event for logging", "event_type", event.EventType(), "error", err)
		return
	}

	recipeID := extractRecipeID(event)
	entry := &domain.EventLogEntry{
		ID:        uuid.New().String(),
		EventType: event.EventType(),
		RecipeID:  recipeID,
		Payload:   string(payload),
		CreatedAt: time.Now().UTC(),
	}

	if err := b.eventLog.Log(ctx, entry); err != nil {
		slog.Error("failed to persist event", "event_type", event.EventType(), "error", err)
	}
}

func extractRecipeID(event domain.Event) string {
	switch e := event.(type) {
	case domain.RecipeSubmitted:
		return e.RecipeID
	case domain.RecipeProcessed:
		return e.RecipeID
	case domain.RecipeProcessingFailed:
		return e.RecipeID
	default:
		return ""
	}
}

func (b *InMemoryEventBus) dispatch(ctx context.Context, event domain.Event) {
	b.mu.RLock()
	handlers := b.handlers[event.EventType()]
	b.mu.RUnlock()

	for _, handler := range handlers {
		if err := handler(ctx, event); err != nil {
			slog.Error("event handler failed",
				"event_type", event.EventType(),
				"error", err,
			)
		}
	}
}
