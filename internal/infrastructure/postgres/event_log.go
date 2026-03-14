package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bbroerse/recipe-processor/internal/domain"
)

type EventLogRepository struct {
	pool *pgxpool.Pool
}

func NewEventLogRepository(pool *pgxpool.Pool) *EventLogRepository {
	return &EventLogRepository{pool: pool}
}

func (r *EventLogRepository) Log(ctx context.Context, entry *domain.EventLogEntry) error {
	ctx, cancel := context.WithTimeout(ctx, dbTimeout)
	defer cancel()

	_, err := r.pool.Exec(ctx,
		`INSERT INTO event_log (id, event_type, recipe_id, payload, created_at)
		 VALUES ($1, $2, $3, $4, $5)`,
		entry.ID, entry.EventType, entry.RecipeID, entry.Payload, entry.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("logging event: %w", err)
	}
	return nil
}
