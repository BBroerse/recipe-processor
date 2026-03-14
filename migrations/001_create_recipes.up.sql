CREATE TABLE IF NOT EXISTS recipes (
    id         TEXT PRIMARY KEY,
    raw_input  TEXT NOT NULL,
    raw_response TEXT DEFAULT '',
    status     TEXT NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_recipes_status ON recipes (status);
