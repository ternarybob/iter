# Iter - Implementation Guide

## Build Commands

```bash
go build ./...
go test ./...
go test -race ./...
golangci-lint run
```

## Project Structure

```
github.com/ternarybob/iter/
├── iter.go                 # Main entry point
├── pkg/
│   ├── sdk/                # Public SDK interfaces
│   ├── agent/              # Core agent implementation
│   ├── orchestra/          # Multi-agent orchestration
│   ├── llm/                # LLM provider abstraction
│   ├── index/              # Codebase indexing
│   ├── config/             # Configuration
│   ├── session/            # Session management
│   ├── monitor/            # Live monitoring
│   └── safety/             # Safety controls
├── skills/                 # Default skills
│   ├── codemod/
│   ├── test/
│   ├── review/
│   ├── patch/
│   ├── devops/
│   └── docs/
└── internal/               # Private utilities
```

## Architecture Rules

### Package Dependencies (import order)
- `pkg/sdk`: No internal dependencies (pure interfaces)
- `pkg/agent`: Depends on sdk, llm, index, config, safety, orchestra
- `pkg/orchestra`: Depends on sdk, llm
- `pkg/llm`: Depends on sdk (for types only)
- `pkg/index`: No pkg dependencies
- `pkg/config`: No pkg dependencies
- `pkg/safety`: No pkg dependencies
- `pkg/monitor`: Depends on sdk (for events)
- `skills/`: Depends on pkg/sdk only

### Code Style
- Use slog for structured logging
- Wrap errors with context: `fmt.Errorf("context: %w", err)`
- Use functional options for configuration
- Interfaces in consumer packages, implementations in provider packages
- No package-level state (except registries with sync.Mutex)

### Testing
- Table-driven tests preferred
- Use testify/assert and testify/require
- Mock external dependencies
- No network calls in unit tests

### Forbidden
- `fmt.Println` (use logger)
- `log.*` (use slog)
- Ignoring errors with `_`
- Global mutable state
- `import "."`
- Circular dependencies

## Multi-Agent Implementation

### Architect Agent
- Uses planning model (higher reasoning)
- Outputs: `requirements.md`, `step_N.md`, `architect-analysis.md`
- Must analyze existing patterns before planning
- Must identify cleanup targets
- Must specify dependencies between steps

### Worker Agent
- Uses execution model (faster, cheaper)
- Follows step docs EXACTLY - no interpretation
- Must verify build passes (output to log file)
- Must perform cleanup specified in step doc
- Writes `step_N_impl.md` after implementation

### Validator Agent
- Uses validation model (higher reasoning)
- DEFAULT STANCE: **REJECT**
- Must verify requirements with code line references
- Must verify cleanup completed
- Auto-reject on build failure
- Auto-reject on missing requirements traceability
- Writes `step_N_valid.md` with verdict

### Final Validator Agent
- Reviews ALL changes together
- Checks for cross-step conflicts
- Verifies all requirements satisfied
- Full build + test must pass
- Writes `final_validation.md`

## Execution Rules

### Absolutes
- CORRECTNESS over SPEED - never rush validation
- Requirements are LAW - no interpretation or deviation
- EXISTING PATTERNS ARE LAW - match codebase style exactly
- CLEANUP IS MANDATORY - remove dead/redundant code
- STEPS ARE MANDATORY - no implementation without step docs
- SUMMARY IS MANDATORY - task incomplete without summary.md
- BUILD VERIFICATION IS MANDATORY - verify after each change
- OUTPUT CAPTURE IS MANDATORY - all command output to log files

### Exit Detection
- Dual condition: indicators >= threshold AND ExitSignal = true
- Never exit on ExitSignal alone
- Never exit on indicators alone
- summary.md existence is a completion indicator

### Output Capture
- ALL build/test output goes to `workdir/logs/`
- Agent context only sees pass/fail + last 30 lines on failure
- Never paste full log contents into context
- Reference logs by path

## Key Types

### Task
```go
type Task struct {
    ID          string
    Description string
    Type        TaskType
    Files       []string
    Context     map[string]any
    Constraints TaskConstraints
}
```

### Result
```go
type Result struct {
    TaskID     string
    SkillName  string
    Status     ResultStatus
    Message    string
    Changes    []Change
    ExitSignal bool
    NextTasks  []*Task
}
```

### Step
```go
type Step struct {
    Number             int
    Title              string
    Dependencies       []int
    Requirements       []string
    Approach           string
    Cleanup            []CleanupItem
    AcceptanceCriteria []string
}
```

### Verdict
```go
type Verdict struct {
    Status            VerdictStatus  // pass or reject
    Reasons           []string
    RequirementStatus map[string]bool
    BuildPassed       bool
    TestsPassed       bool
    CleanupVerified   bool
}
```

## Common Tasks

### Add a new skill
1. Create package in `skills/<name>/`
2. Implement `sdk.Skill` interface
3. Add to `skills/skills.go` All() function
4. Add tests in `skills/<name>/<name>_test.go`

### Add a new LLM provider
1. Implement `llm.Provider` interface in `pkg/llm/`
2. Add constructor like `NewXProvider()`
3. Add option like `WithXProvider()` to `iter.go`
4. Add tests

### Modify the agent loop
1. Core loop logic is in `pkg/agent/loop.go`
2. State machine in `pkg/agent/state.go`
3. Exit detection in `pkg/agent/exit.go`
4. Safety controls in `pkg/agent/circuit.go` and `ratelimit.go`

### Add configuration option
1. Add to `sdk.Config` types in `pkg/sdk/context.go`
2. Add to `FileConfig` in `pkg/config/config.go`
3. Add merge logic in `mergeConfig()`
4. Add option function in `pkg/agent/options.go`
5. Add convenience wrapper in `iter.go`
