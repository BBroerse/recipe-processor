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
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	eventsPublishedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "events_published_total", Help: "Total number of events published.",
	}, []string{"type"})
	eventsProcessedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "events_processed_total", Help: "Total number of events processed by handlers.",
	}, []string{"type", "status"})
	eventsProcessingDurationSeconds = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name: "events_processing_duration_seconds", Help: "Histogram of event processing durations in seconds.",
		Buckets: []float64{0.01, 0.05, 0.1, 0.5, 1, 5, 10, 30, 60},
	}, []string{"type"})
	eventsQueueDepth = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "events_queue_depth", Help: "Current number of events waiting in the event bus channel.",
	})
)

type InMemoryEventBus struct {
	handlers map[string][]domain.EventHandler
	eventLog domain.EventLogRepository
	ch       chan domain.Event
	wg       sync.WaitGroup
	mu       sync.RWMutex
}

func New(bufferSize int, eventLog domain.EventLogRepository) *InMemoryEventBus {
	return &InMemoryEventBus{
		handlers: make(map[string][]domain.EventHandler),
		eventLog: eventLog,
		ch:       make(chan domain.Event, bufferSize),
	}
}

func (b *InMemoryEventBus) Publish(_ context.Context, event domain.Event) error {
	select {
	case b.ch <- event:
		eventsPublishedTotal.WithLabelValues(event.EventType()).Inc()
		eventsQueueDepth.Set(float64(len(b.ch)))
		return nil
	default:
		return fmt.Errorf("event bus buffer full, dropping event: %s", event.EventType())
	}
}

func (b *InMemoryEventBus) Subscribe(eventType string, handler domain.EventHandler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers[eventType] = append(b.handlers[eventType], handler)
}

func (b *InMemoryEventBus) Start(ctx context.Context) error {
	b.wg.Add(1)
	go func() { // #nosec G118 -- long-lived worker goroutine, not request-scoped
		defer b.wg.Done()
		for {
			select {
			case event := <-b.ch:
				b.persistEvent(ctx, event)
				b.dispatch(ctx, event)
			case <-ctx.Done():
				// Drain remaining events with a fresh context since the parent is cancelled
				drainCtx := context.Background() //nolint:contextcheck // intentional: parent ctx is cancelled
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

	eventsQueueDepth.Set(float64(len(b.ch)))

	for _, handler := range handlers {
		start := time.Now()
		if err := handler(ctx, event); err != nil {
			eventsProcessedTotal.WithLabelValues(event.EventType(), "failed").Inc()
			slog.Error("event handler failed",
				"event_type", event.EventType(),
				"error", err,
			)
		} else {
			eventsProcessedTotal.WithLabelValues(event.EventType(), "success").Inc()
		}
		eventsProcessingDurationSeconds.WithLabelValues(event.EventType()).Observe(time.Since(start).Seconds())
	}
}
