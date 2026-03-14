# Copilot Instructions — recipe-processor

## Project Overview

Go API that accepts recipe text, processes it through a local LLM (Ollama), stores structured results in PostgreSQL, and syncs to external visualization tools (Notion). Uses event-driven architecture for async processing.

## Architecture

**Clean Architecture** with strict dependency direction (inward toward domain):

- `cmd/api/` — Entrypoint, wires everything together
- `internal/domain/` — Core entities (`ProcessedRecipe`), domain events, repository interfaces, value objects. No external dependencies allowed.
- `internal/application/` — Use cases (command/event handlers), DTOs. Depends only on domain interfaces.
- `internal/infrastructure/` — PostgreSQL repos, Ollama client, Notion syncer, HTTP handlers, event bus. Implements domain interfaces.
- `internal/shared/` — Event bus interface, middleware (logging, validation, recovery), config, utilities

### Request Flow

```
POST /recipes → validate input → return 202 Accepted
  → publish RecipeSubmitted event
  → [async] LLM handler: call Ollama → parse response → save to PostgreSQL
  → publish RecipeProcessed event
  → [async] Notion handler: sync to Notion (with retry/backoff)
```

The API returns immediately. LLM processing and external sync happen asynchronously via the event bus (in-memory channel-based, upgrade path to Redis Streams/RabbitMQ).

### Key Interfaces

External dependencies are behind interfaces for swappability and testing:

- `LLMProvider` — implemented by `OllamaClient`
- `RecipeSyncer` — implemented by `NotionSyncer`
- `RecipeRepository` — implemented by PostgreSQL repository

## Build & Run

```sh
go build ./...
go run ./cmd/api           # start the API server
go test -short ./...       # unit tests only (fast)
go test ./...              # all tests including integration (needs Docker)
go test ./internal/domain  # run tests for a single package
go test -run TestNewRecipeText_ValidInput ./internal/domain  # run a single test
golangci-lint run          # lint
gosec ./...                # security scan
govulncheck ./...          # vulnerability check
```

### Docker

```sh
docker compose up --build  # API + PostgreSQL + Ollama (first run pulls tinyllama)
OLLAMA_MODEL=mistral docker compose up --build  # use a different model
```

## Dev Workflow

1. Create a feature branch from `main`
2. Make changes, run `go test -short ./...` and `golangci-lint run` locally
3. Push and open a PR against `main`
4. CI runs automatically: lint → security → unit tests → integration tests → build → Docker build
5. On merge to `main`: Docker image is pushed to GHCR with `latest` and commit SHA tags

## Conventions

### Non-Negotiable Rules

- **Every external call MUST have a context timeout** — HTTP: 30s, LLM: 120s, DB: 5s, Notion: 15s
- **Every external call MUST have rate limiting** — Use `golang.org/x/time/rate`
- **Pure functions wherever possible** — no side effects when avoidable
- **Dependency injection** — no global variables, inject dependencies explicitly
- **Errors must be wrapped** — `fmt.Errorf("context: %w", err)` to preserve the chain
- **Pass context through the call chain** — for cancellation propagation
- **All exported functions must have comments**

### Testing

- Table-driven tests for validation and business logic
- Unit tests in their own `_test` package
- Manual mocks via interfaces (no mocking frameworks)
- Integration tests use testcontainers for real PostgreSQL
- HTTP handler tests use `httptest`
- Domain: 90%+ coverage, Application: 80%+, Infrastructure: 70%+

### Error Handling

- Domain errors are sentinel values (`ErrInvalidRecipeText`, `ErrRecipeNotFound`)
- API returns structured `ErrorResponse{Error, Code, Details}`
- Retry with exponential backoff for: network errors, 429s, 5xx — never for validation/4xx errors
- Handle errors once at boundaries; don't log-and-rethrow

### Logging

- Structured JSON via `log/slog`
- Log all HTTP requests, errors with context, external call timing, event processing outcomes

### Database

- **sqlc** for type-safe query generation from SQL (future)
- **golang-migrate/migrate** for versioned migrations in `migrations/`
- `recipes` table: raw input + structured fields (title, ingredients, instructions, total_time, servings, course_type) + status tracking
- `event_log` table: append-only log of all domain events (JSONB payload) for debugging and audit

### Configuration

All config via environment variables (see AGENTS.md for full list). Key ones:
`PORT`, `DB_HOST/PORT/USER/PASSWORD/NAME`, `OLLAMA_URL`, `OLLAMA_MODEL`, `NOTION_API_KEY`, `NOTION_DATABASE_ID`, `LOG_LEVEL`
