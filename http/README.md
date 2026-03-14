# HTTP Test Files

Manual API test scenarios using `.http` files.

## Setup

Install the [REST Client](https://marketplace.visualstudio.com/items?itemName=humao.rest-client) extension for VS Code, or use IntelliJ's built-in HTTP Client.

## Environments

Edit `http-client.env.json` to configure:

| Environment | Base URL | Auth |
|---|---|---|
| `dev` | `http://localhost:8080` | No API key (dev mode) |
| `docker` | `http://localhost:8080` | API key required |

Switch environments in VS Code via the status bar or `Ctrl+Shift+P` → "Rest Client: Switch Environment".

## Files

| File | Scenarios |
|---|---|
| `recipes.http` | Submit recipes (valid, empty, XSS, prompt injection), get by ID |
| `auth.http` | Authentication: missing key, wrong key, correct key |
| `health.http` | Health check endpoint |

## curl alternative

```sh
# Health check
curl http://localhost:8080/health

# Submit recipe (no auth)
curl -X POST http://localhost:8080/recipes \
  -H "Content-Type: application/json" \
  -d '{"text": "Pannenkoeken: 300g bloem, 4 eieren, 500ml melk..."}'

# Submit recipe (with auth)
curl -X POST http://localhost:8080/recipes \
  -H "Content-Type: application/json" \
  -H "X-API-Key: your-key" \
  -d '{"text": "Recipe text here"}'

# Get recipe
curl http://localhost:8080/recipes/<uuid> -H "X-API-Key: your-key"
```
