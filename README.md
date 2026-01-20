# Iter

**Iter** (Latin: "journey, path") is a **Claude Code plugin** that implements an adversarial multi-agent DevOps loop.

It provides a rigorous Architect → Worker → Validator feedback cycle for complex development tasks, with the Validator taking an adversarial stance (default: REJECT) to ensure correctness.

## Features

- **Claude Code Plugin**: Integrates directly with Claude Code via hooks and slash commands
- **Adversarial Validation**: Multi-agent architecture with hostile review (default REJECT)
- **Go-Powered CLI**: State management and prompt generation via compiled Go binary
- **Codebase-Aware**: Built-in AST indexing and semantic search (SDK components)
- **Correctness over Speed**: Requirements are law, validation is mandatory
- **Self-Referential Loop**: Stop hook creates continuous improvement cycle

## Quick Start (Plugin)

```bash
# Clone the repository
git clone https://github.com/ternarybob/iter.git
cd iter

# Build the CLI binary
go build -o bin/iter ./cmd/iter

# Use with Claude Code
claude --plugin-dir /path/to/iter

# Start an adversarial loop
/iter-loop "Add a health check endpoint to the API"
```

## Plugin Commands

| Command | Description |
|---------|-------------|
| `/iter-loop "<task>"` | Start an adversarial multi-agent loop |
| `/iter-analyze` | Run architect analysis |
| `/iter-validate` | Run validator review (adversarial) |
| `/iter-step [N]` | Get step instructions |
| `/iter-status` | Show session status |
| `/iter-next` | Move to next step |
| `/iter-complete` | Mark session complete |
| `/iter-reset` | Reset session |

## Multi-Agent Architecture

```
┌─────────────┐
│  ARCHITECT  │ ─── Analyzes requirements, creates step documents
└──────┬──────┘
       │
       ▼
┌─────────────┐
│   WORKER    │ ─── Implements step exactly (you, via Claude Code)
└──────┬──────┘
       │
       ▼
┌─────────────┐     ┌─────────┐
│  VALIDATOR  │ ──► │ REJECT  │ ──► Back to WORKER (fix issues)
└──────┬──────┘     └─────────┘
       │
       ▼ (PASS)
┌─────────────┐
│  Next Step  │ ──► Repeat until all steps done
└─────────────┘
```

| Agent | Role | Stance |
|-------|------|--------|
| **ARCHITECT** | Analyze requirements, create step docs | Thorough, comprehensive |
| **WORKER** | Implement steps exactly as specified | Follow spec precisely |
| **VALIDATOR** | Review implementation against requirements | **HOSTILE - default REJECT** |

---

## SDK Usage (Embeddable Library)

Iter also provides a Go SDK for building autonomous agents programmatically.

## Installation

### Prerequisites

- **Go 1.22+** - [Download Go](https://go.dev/dl/)
- **Anthropic API Key** - Get one at [console.anthropic.com](https://console.anthropic.com/)
- (Optional) **Ollama** - For local model support: [ollama.ai](https://ollama.ai/)

### Install as a Dependency

Add Iter to your Go project:

```bash
go get github.com/ternarybob/iter
```

### Build from Source

Clone the repository and build:

```bash
# Clone the repository
git clone https://github.com/ternarybob/iter.git
cd iter

# Download dependencies
go mod download

# Build
go build ./...

# Run tests
go test ./...

# Run tests with race detection
go test -race ./...
```

### Verify Installation

```bash
# Check that the module builds correctly
go build ./...

# Run the test suite
go test ./...
```

Expected output:
```
ok  	github.com/ternarybob/iter/pkg/agent
ok  	github.com/ternarybob/iter/pkg/config
ok  	github.com/ternarybob/iter/pkg/index
ok  	github.com/ternarybob/iter/pkg/llm
ok  	github.com/ternarybob/iter/pkg/orchestra
ok  	github.com/ternarybob/iter/pkg/sdk
ok  	github.com/ternarybob/iter/pkg/session
ok  	github.com/ternarybob/iter/skills
```

### Environment Setup

Set your API key as an environment variable:

```bash
# For Anthropic Claude
export ANTHROPIC_API_KEY="your-api-key-here"

# Or add to your shell profile (~/.bashrc, ~/.zshrc, etc.)
echo 'export ANTHROPIC_API_KEY="your-api-key-here"' >> ~/.bashrc
source ~/.bashrc
```

For Ollama (local models), ensure the Ollama server is running:

```bash
# Start Ollama server
ollama serve

# Pull a model (e.g., llama2)
ollama pull llama2
```

## Quick Start

```go
package main

import (
    "context"
    "log"
    "os"

    "github.com/ternarybob/iter"
)

func main() {
    // Create agent with Anthropic API
    agent, err := iter.New(
        iter.WithAnthropicKey(os.Getenv("ANTHROPIC_API_KEY")),
        iter.WithWorkDir("."),
        iter.WithClaudeConfig("."),
    )
    if err != nil {
        log.Fatal(err)
    }

    // Create a task
    task := iter.NewTask("Add a health check endpoint to the API")

    // Execute
    ctx := context.Background()
    result, err := agent.Run(ctx, task)
    if err != nil {
        log.Fatal(err)
    }

    log.Printf("Result: %s", result.Message)
}
```

## Multi-Agent Architecture

Iter uses a multi-agent adversarial architecture:

```
┌─────────────────────────────────────────────────────────────────┐
│                    MULTI-AGENT WORKFLOW                          │
│                                                                  │
│  ARCHITECT  →  WORKER  →  VALIDATOR  →  (iterate if rejected)   │
│  (planning)    (impl)     (hostile)                              │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

| Agent | Role | Stance |
|-------|------|--------|
| **ARCHITECT** | Analyze requirements, create step docs | Thorough, comprehensive |
| **WORKER** | Implement steps exactly as specified | Follow spec precisely |
| **VALIDATOR** | Review implementation against requirements | **HOSTILE - default REJECT** |

### Adversarial Validation

The Validator assumes ALL implementations are incorrect until proven otherwise:

**Auto-reject conditions:**
- Build fails
- Tests fail
- Requirements not traceable to code
- Dead code left behind
- Missing cleanup as specified

**Pass conditions (ALL must be true):**
- ALL requirements verified with code references
- Build passes
- Tests pass
- No dead code
- Cleanup verified

## Configuration

Iter supports the `.claude` directory convention:

```
.claude/
├── settings.json           # Project settings
├── commands/               # Slash commands
├── skills/                 # Custom skill definitions
│   └── custom-skill/
│       └── SKILL.md
└── workdir/                # Iteration artifacts
```

### settings.json

```json
{
  "project": {
    "name": "my-project",
    "ignorePatterns": ["vendor/", "node_modules/"],
    "indexPatterns": ["*.go", "*.ts"]
  },
  "models": {
    "planning": "claude-sonnet-4-20250514",
    "execution": "claude-sonnet-4-20250514",
    "validation": "claude-sonnet-4-20250514"
  },
  "loop": {
    "maxIterations": 100,
    "rateLimitPerHour": 100,
    "maxValidationRetries": 5
  }
}
```

## Custom Skills

Skills are the primary extension point. Implement the `Skill` interface:

```go
type Skill interface {
    Metadata() SkillMetadata
    CanHandle(ctx, ExecutionContext, Task) (bool, float64)
    Plan(ctx, ExecutionContext, Task) (Plan, error)
    Execute(ctx, ExecutionContext, Plan) (Result, error)
    Validate(ctx, ExecutionContext, Result) error
}
```

Or use the functional helper:

```go
skill := sdk.NewSkillFunc(sdk.SkillMetadata{
    Name:     "my-skill",
    Triggers: []string{"my-trigger"},
}).
OnCanHandle(func(ctx, execCtx, task) (bool, float64) {
    return true, 0.8
}).
OnExecute(func(ctx, execCtx, plan) (*sdk.Result, error) {
    return sdk.NewResult(plan.TaskID, "my-skill").
        WithMessage("Done!"), nil
})

agent.RegisterSkill(skill)
```

## Default Skills

| Skill | Purpose | Triggers |
|-------|---------|----------|
| **codemod** | Modify existing code | fix, refactor, modify, implement |
| **test** | Generate and run tests | test, add tests, coverage |
| **review** | Code review | review, audit, security |
| **patch** | Apply patches, resolve conflicts | patch, merge, cherry-pick |
| **devops** | Docker, K8s, CI/CD | docker, kubernetes, deploy |
| **docs** | Documentation | document, readme, api docs |

## Workdir Artifacts

Each execution creates artifacts in `.claude/workdir/`:

| File | Purpose |
|------|---------|
| `requirements.md` | Extracted requirements |
| `architect-analysis.md` | Patterns, decisions |
| `step_N.md` | Step specifications |
| `step_N_impl.md` | Implementation notes |
| `step_N_valid.md` | Validation results |
| `final_validation.md` | Final review |
| `summary.md` | **MANDATORY** completion summary |

## Safety Controls

### Rate Limiting

Token bucket algorithm with configurable hourly limits.

### Circuit Breaker

Prevents infinite loops by detecting:
- Consecutive iterations with no changes
- Repeated errors
- Output decline

## Project Structure

```
github.com/ternarybob/iter/
├── iter.go                 # Main entry point and convenience functions
├── pkg/
│   ├── sdk/                # Public SDK interfaces (Skill, Task, Result, etc.)
│   ├── agent/              # Core agent implementation and loop controller
│   ├── orchestra/          # Multi-agent orchestration (Architect, Worker, Validator)
│   ├── llm/                # LLM provider abstraction (Anthropic, Ollama)
│   ├── index/              # Codebase indexing and semantic search
│   ├── config/             # Configuration loading (.claude directory)
│   ├── session/            # Session and conversation management
│   └── monitor/            # Live monitoring with SSE streaming
├── skills/                 # Default skills
│   ├── codemod/            # Code modification skill
│   ├── test/               # Test generation skill
│   ├── review/             # Code review skill
│   ├── patch/              # Patch application skill
│   ├── devops/             # DevOps skill (Docker, K8s, CI/CD)
│   └── docs/               # Documentation skill
└── internal/               # Private utilities
```

## API Reference

See the [pkg.go.dev documentation](https://pkg.go.dev/github.com/ternarybob/iter).

## Development

### Running Tests

```bash
# Run all tests
go test ./...

# Run tests with verbose output
go test -v ./...

# Run tests with race detection
go test -race ./...

# Run tests for a specific package
go test ./pkg/agent/...

# Run a specific test
go test -run TestCircuitBreaker ./pkg/agent/
```

### Linting

```bash
# Install golangci-lint
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Run linter
golangci-lint run
```

### Building

```bash
# Build all packages
go build ./...

# Build with optimizations disabled (for debugging)
go build -gcflags="all=-N -l" ./...
```

## Troubleshooting

### Common Issues

**"ANTHROPIC_API_KEY not set"**
```bash
export ANTHROPIC_API_KEY="your-api-key-here"
```

**"connection refused" with Ollama**
```bash
# Ensure Ollama is running
ollama serve
```

**Build errors with missing dependencies**
```bash
go mod tidy
go mod download
```

## Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Make your changes
4. Run tests (`go test ./...`)
5. Commit your changes (`git commit -m 'Add amazing feature'`)
6. Push to the branch (`git push origin feature/amazing-feature`)
7. Open a Pull Request

## License

MIT License - see [LICENSE](LICENSE)
