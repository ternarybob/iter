# Iter

**Iter** (Latin: "journey, path") is a pure Go SDK for building autonomous DevOps agents that iteratively improve codebases.

Unlike CLI wrappers, Iter is designed as an embeddable library with a plugin architecture for extensible skills.

## Features

- **SDK-First**: Embeddable library, not a CLI tool
- **Plugin Architecture**: Skills are first-class interfaces
- **Codebase-Aware**: Built-in AST indexing and semantic search
- **Multi-Model**: Strategic routing (planning vs execution models)
- **Adversarial Validation**: Multi-agent architecture with hostile review
- **Correctness over Speed**: Requirements are law, validation is mandatory
- **Convention over Configuration**: `.claude` directory and `SKILL.md` compatibility

## Installation

```bash
go get github.com/ternarybob/iter
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

## API Reference

See the [pkg.go.dev documentation](https://pkg.go.dev/github.com/ternarybob/iter).

## Requirements

- Go 1.22+
- SQLite (bundled with pure-Go driver)
- Anthropic API key (or Ollama for local models)

## License

MIT License - see [LICENSE](LICENSE)
