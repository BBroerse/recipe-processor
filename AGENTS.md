# Recipe Processing API - Architecture & Standards

## Project Overview
A Go API that accepts recipe text, processes it through a local LLM (Ollama), stores the structured result in PostgreSQL, and syncs to external visualization tools (Notion). Uses event-driven architecture to handle async processing.

## Core Principles

### Non-Negotiable Rules
1. **Pure functions wherever possible** - Functions should not have side effects when avoidable
2. **External calls MUST have timeouts** - Every HTTP call, LLM request, DB query needs context with timeout
3. **External calls MUST have rate limiting** - Protect against overwhelming external services
4. **Database transactions when needed** - Explicit transaction boundaries for multi-step operations
5. **Always use context for cancellation** - Pass context through the call chain
6. **Dependency injection** - No global variables, inject dependencies explicitly
7. **Errors must be wrapped** - Use `fmt.Errorf` with `%w` to maintain error chains
8. **Interfaces for external dependencies** - Keep swappable (LLM provider, visualization tool, etc.)

### Architecture Style
- **Clean Architecture** with clear boundaries between layers
- **Event-Driven** for async operations (LLM processing, external syncing)
- **Modular** design allowing easy replacement of components

## Domain Model

### Core Entity
```go
type ProcessedRecipe struct {
    Title        string   `json:"title"`
    Ingredients  []string `json:"ingredients"`
    Instructions []string `json:"instructions"`
    TotalTime    int      `json:"total_time"`
    Servings     int      `json:"servings"`
    CourseType   string   `json:"course_type"`
}
```

### Domain Events
```go
type RecipeProcessed struct {
    RecipeID   string
    Recipe     ProcessedRecipe
    OccurredAt time.Time
}
```

## Clean Architecture Layers

### 1. Domain Layer (internal/domain/)
**Purpose:** Core business logic and entities

**Contains:**
- Entity definitions (`ProcessedRecipe`)
- Domain events
- Repository interfaces
- Domain errors
- Value objects for validation

**Rules:**
- No external dependencies
- Pure business logic only
- No framework imports

### 2. Application Layer (internal/application/)
**Purpose:** Use cases and orchestration

**Contains:**
- Command handlers (ProcessRecipeCommand)
- Event handlers (handle RecipeProcessed events)
- Application services
- DTOs for input/output

**Rules:**
- Orchestrates domain objects
- Depends only on domain interfaces
- No infrastructure concerns

### 3. Infrastructure Layer (internal/infrastructure/)
**Purpose:** External integrations and technical concerns

**Contains:**
- Database repositories (PostgreSQL implementation)
- LLM client (Ollama)
- External sync clients (Notion API)
- Event bus implementation
- HTTP handlers

**Rules:**
- Implements domain interfaces
- All external calls have timeouts
- All external calls have rate limiting
- Proper error handling and logging

### 4. Shared (internal/shared/)
**Purpose:** Cross-cutting concerns

**Contains:**
- Event bus interface and implementation
- Middleware (logging, validation, recovery)
- Configuration
- Common utilities

## Input Validation Rules

### Recipe Text Validation
1. **Required:** Non-empty string
2. **Max length:** 10,000 characters (to prevent token overflow)
3. **XSS Protection:** Sanitize HTML/script tags
4. **Character validation:** Only allow safe UTF-8 characters
5. **Reject:** SQL injection patterns, shell commands, control characters

### Implementation
```go
// Validation happens in domain layer
type RecipeText struct {
    value string
}

func NewRecipeText(text string) (RecipeText, error) {
    // Validation logic here
}
```

## Event-Driven Flow

### Flow Diagram
```
1. POST /recipes (HTTP Handler)
   ↓
2. Validate input
   ↓
3. Return 202 Accepted immediately
   ↓
4. Publish RecipeSubmitted event
   ↓
5. Event Handler: Call Ollama LLM (async)
   ↓
6. Parse LLM response
   ↓
7. Save to PostgreSQL
   ↓
8. Publish RecipeProcessed event
   ↓
9. Event Handler: Sync to Notion (async)
   ↓
10. Retry on failure with backoff
```

### Why Event-Driven?
- API returns immediately (doesn't wait for slow LLM)
- LLM processing happens in background
- External sync doesn't block the main flow
- Easy to add more event handlers later
- Natural retry mechanism for failures

## Event Bus Design

### Interface
```go
type EventBus interface {
    Publish(ctx context.Context, event Event) error
    Subscribe(eventType string, handler EventHandler)
    Start(ctx context.Context) error
    Stop() error
}

type EventHandler func(ctx context.Context, event Event) error
```

### Implementation Choice
**Start simple:** In-memory channel-based event bus
- Goroutines with worker pools
- Buffered channels to prevent blocking
- Graceful shutdown on context cancellation

**Future upgrade path:** Redis Streams, RabbitMQ, or Kafka if needed

## External Service Interfaces

### LLM Provider
```go
type LLMProvider interface {
    ProcessRecipe(ctx context.Context, text string) (ProcessedRecipe, error)
}

// Implementation: OllamaClient
// Future: OpenAIClient, AnthropicClient, etc.
```

### Visualization Sync
```go
type RecipeSyncer interface {
    Sync(ctx context.Context, recipe ProcessedRecipe) error
}

// Implementation: NotionSyncer
// Future: AirtableSyncer, GoogleSheetsSyncer, etc.
```

### Repository
```go
type RecipeRepository interface {
    Save(ctx context.Context, recipe ProcessedRecipe) (id string, err error)
    FindByID(ctx context.Context, id string) (ProcessedRecipe, error)
    List(ctx context.Context, limit, offset int) ([]ProcessedRecipe, error)
}
```

## Timeout & Rate Limiting Standards

### Timeouts
```go
const (
    HTTPTimeout    = 30 * time.Second  // API requests
    LLMTimeout     = 120 * time.Second // LLM can be slow
    DBTimeout      = 5 * time.Second   // Database queries
    NotionTimeout  = 15 * time.Second  // External API calls
)
```

### Rate Limiting
```go
// Use golang.org/x/time/rate
const (
    LLMRateLimit    = rate.Limit(5)   // 5 req/sec to Ollama
    NotionRateLimit = rate.Limit(3)   // 3 req/sec to Notion API
)
```

### Implementation Pattern
```go
func (c *OllamaClient) ProcessRecipe(ctx context.Context, text string) (ProcessedRecipe, error) {
    // Rate limit
    if err := c.limiter.Wait(ctx); err != nil {
        return ProcessedRecipe{}, fmt.Errorf("rate limit wait: %w", err)
    }
    
    // Timeout
    ctx, cancel := context.WithTimeout(ctx, LLMTimeout)
    defer cancel()
    
    // Actual call
    // ...
}
```

## Error Handling Standards

### Error Types
```go
// Domain errors
var (
    ErrInvalidRecipeText = errors.New("invalid recipe text")
    ErrRecipeNotFound    = errors.New("recipe not found")
    ErrLLMProcessing     = errors.New("llm processing failed")
)

// Wrap errors with context
return fmt.Errorf("failed to save recipe: %w", err)
```

### Error Response Format
```go
type ErrorResponse struct {
    Error   string `json:"error"`
    Code    string `json:"code"`
    Details string `json:"details,omitempty"`
}
```

## Logging Standards

### What to Log
1. **All HTTP requests** - Method, path, status, duration
2. **All responses** - Status code, size
3. **All errors** - With full context and stack trace
4. **Performance metrics** - Operation durations
5. **External calls** - Request/response (sanitized), timing
6. **Event processing** - Event type, handler, duration, success/failure

### Log Format
**Structured JSON logging** using standard library `log/slog`

```go
slog.Info("http request",
    "method", r.Method,
    "path", r.URL.Path,
    "duration_ms", duration.Milliseconds(),
    "status", status,
)
```

### Log Levels
- **DEBUG:** Detailed flow for development
- **INFO:** Normal operations (requests, events processed)
- **WARN:** Recoverable issues (retry attempts)
- **ERROR:** Failures requiring attention

## Database Standards

### ORM Choice
**sqlc** - Generate type-safe Go from SQL
- Write SQL queries (you control them)
- Generated Go code (type-safe)
- No runtime reflection overhead
- Easy to review generated code

Alternative: **sqlx** if you prefer more flexibility

### Migration Tool
**golang-migrate/migrate** - Database migrations
- Version controlled SQL files
- Up/down migrations
- Works in CI/CD

### Schema Flexibility
Keep schema definitions in version-controlled SQL files. Let specific table structures evolve as needed.

## Testing Standards

### Testing Layers

**1. Unit Tests (domain & application)**
- unit tests should always be in their own package with _test
```go
func TestNewRecipeText_ValidInput(t *testing.T) {
    text := "Valid recipe text"
    rt, err := NewRecipeText(text)
    
    if err != nil {
        t.Errorf("expected no error, got %v", err)
    }
    if rt.value != text {
        t.Errorf("expected %s, got %s", text, rt.value)
    }
}
```

**2. Integration Tests (infrastructure)**
- Test with real PostgreSQL (use testcontainers)
- Test HTTP handlers with httptest
- Test event bus with real goroutines

**3. Table-Driven Tests**
```go
func TestRecipeTextValidation(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        wantErr bool
    }{
        {"valid", "Recipe text", false},
        {"empty", "", true},
        {"too long", strings.Repeat("a", 10001), true},
        {"xss", "<script>alert('xss')</script>", true},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            _, err := NewRecipeText(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("wantErr %v, got %v", tt.wantErr, err)
            }
        })
    }
}
```

### Test Coverage Target
- **Domain logic:** 90%+ coverage
- **Application logic:** 80%+ coverage
- **Infrastructure:** 70%+ coverage (focus on critical paths)

### Mocking Strategy
Use interfaces + manual mocks (keep it simple)
```go
type mockLLMProvider struct {
    processFunc func(ctx context.Context, text string) (ProcessedRecipe, error)
}

func (m *mockLLMProvider) ProcessRecipe(ctx context.Context, text string) (ProcessedRecipe, error) {
    return m.processFunc(ctx, text)
}
```

## Configuration Standards

### Environment Variables
```bash
# Server
PORT=8080
ENV=development # development, production

# Database
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=secret
DB_NAME=recipes
DB_MAX_CONNS=25
DB_MAX_IDLE_CONNS=5

# Ollama
OLLAMA_URL=http://localhost:11434
OLLAMA_MODEL=llama2

# Notion
NOTION_API_KEY=secret_xxx
NOTION_DATABASE_ID=xxx

# Logging
LOG_LEVEL=info # debug, info, warn, error
```

### Config Struct
```go
type Config struct {
    Server   ServerConfig
    Database DatabaseConfig
    Ollama   OllamaConfig
    Notion   NotionConfig
    Logging  LoggingConfig
}

func LoadConfig() (*Config, error) {
    // Load from environment variables
    // Validate required fields
    // Return error if missing required config
}
```

## GitHub Actions CI/CD

### Pipeline Structure
```yaml
name: CI/CD

on: [push, pull_request]

jobs:
  # Run in parallel
  lint:
    # golangci-lint
  
  security:
    # gosec, govulncheck
  
  test:
    # go test with coverage
  
  build:
    needs: [lint, security, test]
    # go build
  
  deploy:
    needs: [build]
    if: github.ref == 'refs/heads/main'
    # Deploy to self-hosted machine
```

### Required Checks
1. **Linting:** `golangci-lint` (run multiple linters)
2. **Security:** `gosec` (security issues), `govulncheck` (known vulnerabilities)
3. **Tests:** All tests pass with minimum coverage
4. **Build:** Successful compilation
5. **Docker:** Build Docker image successfully

### Deployment Strategy
- Build Docker image
- Push to registry (GitHub Container Registry or self-hosted)
- SSH to self-hosted machine
- Pull new image
- Restart containers with docker-compose

## Docker Setup

### Multi-Stage Dockerfile
```dockerfile
# Build stage
FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o api cmd/api/main.go

# Runtime stage
FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/api .
EXPOSE 8080
CMD ["./api"]
```

### Docker Compose
```yaml
version: '3.8'

services:
  api:
    build: .
    ports:
      - "8080:8080"
    environment:
      - DB_HOST=postgres
      - OLLAMA_URL=http://ollama:11434
    depends_on:
      - postgres
      - ollama
  
  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_DB: recipes
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: secret
    volumes:
      - postgres_data:/var/lib/postgresql/data
  
  ollama:
    image: ollama/ollama:latest
    volumes:
      - ollama_data:/root/.ollama

volumes:
  postgres_data:
  ollama_data:
```

## Retry Strategy

### Simple Exponential Backoff
```go
func retryWithBackoff(ctx context.Context, operation func() error, maxRetries int) error {
    var err error
    for i := 0; i < maxRetries; i++ {
        err = operation()
        if err == nil {
            return nil
        }
        
        // Exponential backoff: 1s, 2s, 4s, 8s
        backoff := time.Duration(1<<uint(i)) * time.Second
        
        slog.Warn("operation failed, retrying",
            "attempt", i+1,
            "max_retries", maxRetries,
            "backoff", backoff,
            "error", err,
        )
        
        select {
        case <-time.After(backoff):
            continue
        case <-ctx.Done():
            return ctx.Err()
        }
    }
    return fmt.Errorf("max retries exceeded: %w", err)
}
```

### When to Retry
- **Notion API calls:** Network errors, 429 rate limits, 5xx errors
- **Ollama calls:** Network errors (not parsing errors)
- **DO NOT retry:** Validation errors, 4xx client errors (except 429)

## Code Organization Checklist

### Before Committing
- [ ] All functions have comments (exported ones must)
- [ ] Error messages are descriptive and wrapped
- [ ] Timeouts are set for external calls
- [ ] Rate limiting is in place
- [ ] Tests are written and passing
- [ ] No TODO comments in main branch
- [ ] Logging is appropriate (not too verbose, not too sparse)
- [ ] Secrets are not hardcoded

### Code Review Focus
- Is the dependency direction correct? (inward toward domain)
- Are interfaces used for external dependencies?
- Is error handling consistent?
- Are pure functions used where possible?
- Is the context passed through properly?

## Future Extensibility Points

### Easy to Add Later
1. **More LLM providers** - Implement LLMProvider interface
2. **More sync destinations** - Implement RecipeSyncer interface
3. **Authentication** - Add middleware
4. **Caching** - Add Redis for processed recipes
5. **Webhooks** - New event handler
6. **Search** - Add Elasticsearch sync handler
7. **Metrics** - Prometheus middleware
8. **Tracing** - OpenTelemetry integration

### When to Split into Microservices
**Don't do it yet.** Keep as modulith until:
- LLM processing becomes a bottleneck
- Need to scale sync independently
- Team grows beyond 5-10 people
- Deployment complexity is worth it

## Key Mantras

1. **Make it work, make it right, make it fast** - In that order
2. **Explicit is better than implicit** - Clear dependencies, clear flow
3. **Errors should be handled once** - Log and handle at boundaries
4. **Optimize for readability** - Code is read 10x more than written
5. **Test behavior, not implementation** - Test what it does, not how
6. **When in doubt, keep it simple** - Solve today's problems, not tomorrow's

## Getting Started Checklist

### Initial Setup
- [ ] Initialize Go module
- [ ] Set up Docker Compose (API, PostgreSQL, Ollama)
- [ ] Configure environment variables
- [ ] Set up database migrations
- [ ] Implement basic HTTP server
- [ ] Add structured logging
- [ ] Set up GitHub Actions

### Development Flow
1. Write domain logic first (pure, testable)
2. Define interfaces for external dependencies
3. Implement infrastructure (with timeouts, limits)
4. Wire everything together
5. Add tests
6. Run locally with Docker Compose
7. Commit and push (CI runs automatically)
