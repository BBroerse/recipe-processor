CREATE TABLE IF NOT EXISTS event_log (
    id         TEXT PRIMARY KEY,
    event_type TEXT NOT NULL,
    recipe_id  TEXT NOT NULL,
    payload    JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_event_log_recipe_id ON event_log (recipe_id);
CREATE INDEX idx_event_log_event_type ON event_log (event_type);
CREATE INDEX idx_event_log_created_at ON event_log (created_at);
