# Recipe Processor

A Go API that accepts raw recipe text, processes it through a local LLM ([Ollama](https://ollama.com)), and stores structured results in PostgreSQL. Uses event-driven architecture for async processing.

## Architecture

```
POST /recipes → validate → 202 Accepted
  → [async] LLM extracts structured data (title, ingredients, instructions, …)
  → save to PostgreSQL
  → [future] sync to Notion
```

Clean Architecture with strict dependency direction:

```
cmd/api/             → entrypoint, wires everything
internal/domain/     → entities, events, interfaces (no external deps)
internal/application/→ use cases, event handlers
internal/infrastructure/ → PostgreSQL, Ollama client, HTTP handlers
internal/shared/     → event bus, config, middleware
```

## Quick Start

### Docker (recommended)

```sh
cp .env.example .env   # configure secrets
docker compose up --build
```

The API starts on `localhost:8080`. Ollama pulls `tinyllama` on first run (~5 min).

### Local Development

**Prerequisites:** Go 1.25+, PostgreSQL, Ollama running locally

```sh
# Start dependencies
ollama serve &
docker compose up postgres -d

# Run the API
go run ./cmd/api
```

### Try It

```sh
# Submit a recipe
curl -X POST http://localhost:8080/recipes \
  -H "Content-Type: application/json" \
  -d '{"text": "Classic pancakes: mix 2 cups flour, 2 eggs, 1 cup milk. Cook on griddle 3 min per side. Serves 4."}'

# Check health
curl http://localhost:8080/health

# Get a recipe by ID
curl http://localhost:8080/recipes/{id}
```

## API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/recipes` | Submit recipe text for processing (returns 202) |
| `GET` | `/recipes/{id}` | Retrieve a processed recipe |
| `GET` | `/health` | Health check |

## Configuration

All config via environment variables. See [`.env.example`](.env.example) for the full list.

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | API server port |
| `ENV` | `development` | Environment (`development` / `production`) |
| `OLLAMA_URL` | `http://localhost:11434` | Ollama API endpoint |
| `OLLAMA_MODEL` | `tinyllama` | LLM model to use |
| `DB_HOST` | `localhost` | PostgreSQL host |
| `DB_NAME` | `recipes` | Database name |
| `LOG_LEVEL` | `info` | Log level (`debug`, `info`, `warn`, `error`) |

## Development

```sh
# Run unit tests
go test -short ./...

# Run all tests (needs Docker for testcontainers)
go test ./...

# Lint (requires golangci-lint v2)
golangci-lint run

# Security scan
gosec ./...
govulncheck ./...
```

## Project Status

This is an active side project. See open issues and PRs for current work.

## License

This project is licensed under the MIT License — see [LICENSE](LICENSE) for details.
