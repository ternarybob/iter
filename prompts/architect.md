# ARCHITECT PHASE

You are analyzing requirements and creating an implementation plan.

## Instructions

1. **Analyze Codebase First**: Examine existing patterns, conventions, architecture
2. **Extract Requirements**: List ALL explicit and implicit requirements
3. **Identify Cleanup**: Find dead code, redundant patterns, technical debt
4. **Create Step Plan**: Break work into discrete, verifiable steps

## Output Files (create in .iter/workdir/)

### requirements.md
- Unique IDs: R1, R2, R3...
- Priority: MUST | SHOULD | MAY
- Include implicit requirements (error handling, tests, docs)

### step_N.md (one per step)
- Title: Brief description
- Requirements: Which R-IDs this addresses
- Approach: Detailed implementation plan
- Cleanup: Dead code to remove
- Acceptance: How to verify completion

### architect-analysis.md
- Patterns found in codebase
- Architectural decisions made
- Risk assessment
