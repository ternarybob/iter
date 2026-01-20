# Go Skill for Quaero

**Prerequisite:** Read `.claude/skills/refactoring/SKILL.md` first.

## Project Context
- **Language:** Go 1.25+
- **Storage:** BadgerDB (embedded key-value store)
- **Logging:** github.com/ternarybob/arbor
- **Configuration:** TOML via github.com/pelletier/go-toml/v2
- **LLM:** Google ADK with Gemini models

## Package Structure
```
internal/
├── app/          # DI & orchestration
├── common/       # Stateless utilities
├── handlers/     # HTTP & WebSocket handlers
├── services/     # Stateful business services
├── storage/      # Data persistence (Badger)
├── interfaces/   # Service interfaces
├── models/       # Data models
├── queue/        # Badger-backed job queue
└── jobs/         # Job management
```

## Required Patterns

### Error Handling
```go
if err != nil {
    return fmt.Errorf("failed to process %s: %w", id, err)
}
```

### Logging (arbor only)
```go
logger.Info("message", "key", value)
logger.Error("failed", "error", err)
```

### Constructor Injection
```go
func NewService(dep interfaces.Dependency) *Service {
    return &Service{dep: dep}
}
```

### Thin Handlers
```go
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
    result, err := h.service.DoWork(r.Context(), id)
    // Minimal logic - delegate to service
}
```

## Anti-Patterns (AUTO-FAIL)
```go
// ❌ Global state
var db *badger.DB

// ❌ Panic on errors
panic(err)

// ❌ Missing context
func DoWork() error { }

// ❌ Bare errors
return err

// ❌ fmt/log for logging
fmt.Println("debug")

// ❌ Business logic in handlers
// 50+ lines in handler = WRONG

// ❌ Direct go build
go build ./cmd/quaero

// ❌ Dead code left behind
func oldHelper() { }  // Remove if replaced!

// ❌ Unused imports
import "unused/pkg"  // Clean up!
```

## Build & Test
```bash
# Build (ALWAYS use scripts)
.\scripts\build.ps1      # Windows
./scripts/build.sh       # Linux/macOS

# Tests
go test -v ./test/api    # API tests
go test -v ./test/ui     # UI tests
go test ./internal/...   # Unit tests
```

## Rules Summary

1. Use build scripts - never `go build` directly
2. Context everywhere - pass `context.Context` to I/O
3. Structured logging - arbor with key-value pairs
4. Wrap errors - always add context with `%w`
5. Interface-based DI - depend on interfaces
6. Constructor injection - all deps via `NewXxx()`
7. Thin handlers - logic in services
8. **Remove dead code** - don't leave old functions
9. **Clean imports** - remove unused packages