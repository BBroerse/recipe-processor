# Configuration Reference

> **Auto-generated** — do not edit manually. Update the source in `cmd/gendocs/main.go` and run `go generate ./cmd/gendocs`.

All configuration is supplied via environment variables.

## Environment Variables

| Variable | Type | Default | Required | Description |
|----------|------|---------|----------|-------------|
| `ENV` | string | `development` | No | Deployment environment (`development` or `production`). Controls features like Swagger UI availability. |
| `PORT` | int | `8080` | No | HTTP server listen port. |
| `DB_HOST` | string | `localhost` | No | PostgreSQL server hostname. |
| `DB_PORT` | int | `5432` | No | PostgreSQL server port. |
| `DB_USER` | string | `postgres` | No | PostgreSQL username. |
| `DB_PASSWORD` | string | — | **Yes** | PostgreSQL password. **Must be set** — the application will not start without it. |
| `DB_NAME` | string | `recipes` | No | PostgreSQL database name. |
| `DB_SSLMODE` | string | `require` | No | PostgreSQL SSL mode (e.g. `disable`, `require`, `verify-full`). |
| `OLLAMA_URL` | string | `http://localhost:11434` | No | Base URL of the Ollama LLM service. |
| `OLLAMA_MODEL` | string | `tinyllama` | No | Ollama model name used for recipe text processing. |
| `LOG_LEVEL` | string | `info` | No | Logging verbosity level (`debug`, `info`, `warn`, `error`). |

## Quick Start

```bash
# Minimal required configuration
export DB_PASSWORD="your-secure-password"

# Start the server (all other values use defaults)
go run ./cmd/api
```

## Example: Full Configuration

```bash
export ENV=production
export PORT=8080
export DB_HOST=db.example.com
export DB_PORT=5432
export DB_USER=recipe_app
export DB_PASSWORD=super-secret
export DB_NAME=recipes
export DB_SSLMODE=require
export OLLAMA_URL=http://ollama:11434
export OLLAMA_MODEL=tinyllama
export LOG_LEVEL=info
```
