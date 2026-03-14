package eventbus_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/bbroerse/recipe-processor/internal/domain"
	"github.com/bbroerse/recipe-processor/internal/shared/eventbus"
	"github.com/bbroerse/recipe-processor/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEventBus_PublishAndDispatch(t *testing.T) {
	eventLog := testutil.NewMockEventLogRepository()
	bus := eventbus.New(10, eventLog)

	var received []domain.Event
	var mu sync.Mutex

	bus.Subscribe("recipe.submitted", func(_ context.Context, event domain.Event) error {
		mu.Lock()
		defer mu.Unlock()
		received = append(received, event)
		return nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	require.NoError(t, bus.Start(ctx))

	event := domain.RecipeSubmitted{RecipeID: "r1", RawInput: "test", Timestamp: time.Now()}
	require.NoError(t, bus.Publish(ctx, event))

	// Wait for async dispatch
	assert.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(received) == 1
	}, 2*time.Second, 10*time.Millisecond)

	mu.Lock()
	assert.Equal(t, "r1", received[0].(domain.RecipeSubmitted).RecipeID)
	mu.Unlock()
}

func TestEventBus_PersistsEvents(t *testing.T) {
	eventLog := testutil.NewMockEventLogRepository()
	bus := eventbus.New(10, eventLog)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	require.NoError(t, bus.Start(ctx))

	event := domain.RecipeSubmitted{RecipeID: "r1", RawInput: "test", Timestamp: time.Now()}
	require.NoError(t, bus.Publish(ctx, event))

	assert.Eventually(t, func() bool {
		return len(eventLog.GetEntries()) == 1
	}, 2*time.Second, 10*time.Millisecond)

	entry := eventLog.GetEntries()[0]
	assert.Equal(t, "recipe.submitted", entry.EventType)
	assert.Equal(t, "r1", entry.RecipeID)
	assert.NotEmpty(t, entry.Payload)
	assert.NotEmpty(t, entry.ID)
}

func TestEventBus_MultipleHandlers(t *testing.T) {
	eventLog := testutil.NewMockEventLogRepository()
	bus := eventbus.New(10, eventLog)

	var count int
	var mu sync.Mutex
	handler := func(_ context.Context, _ domain.Event) error {
		mu.Lock()
		defer mu.Unlock()
		count++
		return nil
	}

	bus.Subscribe("recipe.submitted", handler)
	bus.Subscribe("recipe.submitted", handler)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	require.NoError(t, bus.Start(ctx))

	require.NoError(t, bus.Publish(ctx, domain.RecipeSubmitted{RecipeID: "r1", Timestamp: time.Now()}))

	assert.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return count == 2
	}, 2*time.Second, 10*time.Millisecond)
}

func TestEventBus_BufferFull(t *testing.T) {
	eventLog := testutil.NewMockEventLogRepository()
	bus := eventbus.New(1, eventLog) // buffer of 1

	// Don't start the bus — nothing drains the channel
	event := domain.RecipeSubmitted{RecipeID: "r1", Timestamp: time.Now()}
	require.NoError(t, bus.Publish(context.Background(), event))

	// Second publish should fail (buffer full)
	err := bus.Publish(context.Background(), domain.RecipeSubmitted{RecipeID: "r2", Timestamp: time.Now()})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "buffer full")
}

func TestEventBus_NoSubscribers(t *testing.T) {
	eventLog := testutil.NewMockEventLogRepository()
	bus := eventbus.New(10, eventLog)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	require.NoError(t, bus.Start(ctx))

	// Publish event with no subscribers — should not panic
	require.NoError(t, bus.Publish(ctx, domain.RecipeProcessed{RecipeID: "r1", Timestamp: time.Now()}))

	// Event still persisted
	assert.Eventually(t, func() bool {
		return len(eventLog.GetEntries()) == 1
	}, 2*time.Second, 10*time.Millisecond)
}

func TestEventBus_DifferentEventTypes(t *testing.T) {
	eventLog := testutil.NewMockEventLogRepository()
	bus := eventbus.New(10, eventLog)

	var submittedCount, processedCount int
	var mu sync.Mutex

	bus.Subscribe("recipe.submitted", func(_ context.Context, _ domain.Event) error {
		mu.Lock()
		defer mu.Unlock()
		submittedCount++
		return nil
	})
	bus.Subscribe("recipe.processed", func(_ context.Context, _ domain.Event) error {
		mu.Lock()
		defer mu.Unlock()
		processedCount++
		return nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	require.NoError(t, bus.Start(ctx))

	require.NoError(t, bus.Publish(ctx, domain.RecipeSubmitted{RecipeID: "r1", Timestamp: time.Now()}))
	require.NoError(t, bus.Publish(ctx, domain.RecipeProcessed{RecipeID: "r1", Timestamp: time.Now()}))

	assert.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return submittedCount == 1 && processedCount == 1
	}, 2*time.Second, 10*time.Millisecond)
}
