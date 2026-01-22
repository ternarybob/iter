# Code Indexing Integration Recommendations

This document outlines how the `/iter` command can better leverage the code indexing system (`/iter-index`) to improve implementation quality, reduce errors, and ensure comprehensive testing.

## Current State

Currently, `/iter` auto-starts the index daemon and provides access to `/iter-search` for manual queries. However, the indexing system is **not deeply integrated** into the ARCHITECT, WORKER, or VALIDATOR phases—agents must manually invoke searches.

## Recommendations

### 1. Architect Phase: Index-Informed Task Planning

The ARCHITECT phase should automatically query the index to understand code structure before planning.

#### 1.1 Locate Relevant Code

**Problem:** The architect may plan changes without knowing where related code exists.

**Recommendation:** Before creating step documents, the architect should:
```bash
# Find all functions/methods related to the task
/iter-search "<task keywords>" --limit=20

# Locate existing implementations
/iter-search "Handler" --kind=function --path=internal/
```

**Proposed Enhancement:** Inject index search results directly into the ARCHITECT prompt:
- Extract keywords from task description
- Pre-query the index for relevant symbols
- Include top 10 matches in the context

#### 1.2 Discover Dependencies

**Problem:** Changes to one function may require updates to callers/callees.

**Recommendation:** The index should track:
- **Function call graphs**: Which functions call which
- **Import relationships**: Package dependencies
- **Interface implementations**: All types implementing an interface

**Current Gap:** The index stores individual symbols but lacks relationship data.

**Proposed Enhancement:** Add dependency extraction to the parser:
```go
// In index/parser.go - extract function calls
type Chunk struct {
    // ... existing fields
    Calls      []string  // Functions this symbol calls
    CalledBy   []string  // Functions that call this symbol (populated via analysis)
    Implements string    // Interface this type implements
}
```

#### 1.3 Find All References

**Problem:** Renaming or modifying a function signature requires updating all call sites.

**Recommendation:** Before planning changes, query for all usages:
```bash
# Find all places that call a function
/iter-search "ProcessRequest" --kind=function
# Then grep for actual call sites
grep -rn "ProcessRequest(" --include="*.go"
```

**Proposed Enhancement:** Add a `--references` flag to search:
```bash
/iter-search "ProcessRequest" --references
# Returns all files/lines where symbol is used
```

#### 1.4 Prevent Code Duplication

**Problem:** New functions may duplicate existing functionality.

**Recommendation:** Before creating new functions, check for similar signatures:
```bash
# Search for functions with similar input/output patterns
/iter-search "Parse" --kind=function
/iter-search "Config" --kind=type
```

**Proposed Enhancement:** Add signature-based similarity search:
```bash
/iter-search --signature="func(string) error" --limit=10
# Returns functions matching the signature pattern
```

---

### 2. Impact Analysis and Testing

#### 2.1 Identify Impacted Scripts

**Problem:** Code changes may break scripts that invoke the binary or import packages.

**Recommendation:** Index should include:
- Shell scripts (`.sh`, `.bash`) that call the binary
- Makefiles and build scripts
- GitHub Actions workflows
- Integration test scripts

**Proposed Enhancement:** Extend parser to handle non-Go files:
```go
// index/script_parser.go
func ParseScript(path string) ([]Chunk, error) {
    // Extract command invocations: iter run, iter search, etc.
    // Extract environment variables used
    // Extract file paths referenced
}
```

#### 2.2 Automatic Test Discovery

**Problem:** Workers and validators should run relevant tests, but may not know which tests to run.

**Recommendation:** The index should map:
- `*_test.go` files to their corresponding source files
- Test function names to the functions they test
- Integration test scripts to the components they exercise

**Proposed Enhancement:** Add test relationship metadata:
```bash
# Query for tests related to a function
/iter-search "ProcessRequest" --tests
# Returns: TestProcessRequest, TestProcessRequestError, etc.
```

#### 2.3 Script Execution Validation

**Problem:** Changes may break scripts that use the binary.

**Recommendation:** After code changes, automatically run:
```bash
# Discover and run relevant test scripts
find scripts/ -name "*.sh" -exec grep -l "iter" {} \;
# Run each matching script in a test context
```

**Proposed Enhancement:** Add script validation to VALIDATOR phase:
```markdown
### Validation Checklist (enhanced)

**6. SCRIPT COMPATIBILITY** (AUTO-REJECT if fails)
- Identify scripts impacted by changes
- Run affected scripts with test inputs
- Verify exit codes and output
```

---

### 3. Test Integration

#### 3.1 Automatic Test Execution

**Problem:** The VALIDATOR phase mentions "run tests" but doesn't specify which tests.

**Recommendation:** The `/iter` command should:
1. Query index for test files related to changed code
2. Automatically determine the test command (`go test`, `npm test`, etc.)
3. Run only relevant tests for faster feedback

**Current Gap:** The prompts say "run tests" but don't provide test discovery.

**Proposed Enhancement:** Add to WORKER and VALIDATOR prompts:
```markdown
### Test Execution

1. Query changed symbols: `iter search "<changed_function>"`
2. Find related tests: Look for `Test<FunctionName>` in `*_test.go`
3. Run targeted tests: `go test -v -run "TestFunctionName" ./path/to/pkg/...`
4. If no specific tests found, run full test suite: `go test ./...`
```

#### 3.2 Test Coverage Tracking

**Problem:** New code may lack test coverage.

**Recommendation:** Track test coverage for indexed symbols:
```bash
# Check if a function has corresponding tests
/iter-search "ProcessRequest" --coverage
# Returns: Covered by TestProcessRequest (85% coverage)
```

#### 3.3 Integration with Test Scripts

**Problem:** Scripts in `scripts/` directory may contain important tests.

**Recommendation:** Index should catalog test-related scripts:
```bash
scripts/build.sh        # Build verification
scripts/test.sh         # Full test suite
scripts/lint.sh         # Linting
tests/integration/*.sh  # Integration tests
```

**Proposed Enhancement:** Add script discovery to session start:
```
## Available Test Commands

Discovered test commands for this project:
- Build: `./scripts/build.sh`
- Unit Tests: `go test ./...`
- Integration: `./scripts/test-integration.sh`
- Lint: `golangci-lint run`

Run all validation: `./scripts/build.sh && go test ./... && golangci-lint run`
```

---

## Implementation Priority

| Priority | Enhancement | Effort | Impact |
|----------|-------------|--------|--------|
| **P0** | Inject index results into ARCHITECT prompt | Low | High |
| **P0** | Auto-discover test commands on session start | Low | High |
| **P1** | Add `--references` flag to search | Medium | High |
| **P1** | Index shell scripts for command usage | Medium | Medium |
| **P2** | Dependency graph extraction | High | High |
| **P2** | Signature-based similarity search | Medium | Medium |
| **P3** | Test coverage integration | High | Medium |

---

## Summary

The code index is a powerful tool that is currently **underutilized** by `/iter`. Key improvements:

1. **ARCHITECT Phase**: Automatically query index for relevant code, dependencies, and potential duplicates before planning
2. **Impact Analysis**: Index scripts and configuration files to identify all affected artifacts
3. **Test Integration**: Discover and run relevant tests automatically during validation

These enhancements would transform `/iter` from a process that relies on manual code discovery to one that proactively understands the codebase structure and ensures comprehensive change validation.

