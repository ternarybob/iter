---
description: Adversarial multi-agent DevOps workflow with Architect, Worker, and Validator agents
triggers:
  - implement
  - build
  - refactor
  - feature
  - fix complex
  - multi-step
---

# Adversarial DevOps Skill

This skill implements a rigorous, adversarial multi-agent workflow for complex tasks.

## When to Use

Use this skill for:
- Multi-step implementations
- Complex refactoring
- Features requiring careful planning
- Tasks where correctness is critical

## Architecture

### Three Adversarial Agents

1. **ARCHITECT** - Plans with high reasoning
   - Analyzes existing patterns FIRST
   - Creates detailed requirements
   - Breaks work into verifiable steps
   - Identifies cleanup targets

2. **WORKER** - Executes precisely
   - Follows step docs EXACTLY
   - No interpretation or deviation
   - Verifies build after each change
   - Documents implementation

3. **VALIDATOR** - Reviews adversarially
   - **DEFAULT STANCE: REJECT**
   - Must find problems
   - Auto-reject on build failure
   - Auto-reject on missing traceability

## Workflow

```
┌─────────────┐
│  ARCHITECT  │ ─── Creates requirements.md, step_N.md
└──────┬──────┘
       │
       ▼
┌─────────────┐
│   WORKER    │ ─── Implements step exactly
└──────┬──────┘
       │
       ▼
┌─────────────┐     ┌─────────┐
│  VALIDATOR  │ ──► │ REJECT  │ ──► Back to WORKER
└──────┬──────┘     └─────────┘
       │
       ▼ (PASS)
┌─────────────┐
│  Next Step  │ ──► Repeat until all steps done
└─────────────┘
```

## Starting the Workflow

```bash
/iter-loop "Your complex task description"
```

## Artifacts Created

All artifacts go in `.iter/workdir/`:

- `requirements.md` - All requirements with IDs
- `step_N.md` - Step documents
- `step_N_impl.md` - Implementation notes
- `step_N_valid.md` - Validation results
- `architect-analysis.md` - Initial analysis
- `summary.md` - Final summary

## Critical Rules

1. **CORRECTNESS over SPEED** - Never rush validation
2. **Requirements are LAW** - No interpretation
3. **Existing patterns are LAW** - Match style exactly
4. **Cleanup is MANDATORY** - Remove dead code
5. **Build verification is MANDATORY** - After each change

## Exit Conditions

The loop exits when:
- All steps pass validation AND `/iter-complete` is run
- Max iterations reached
- User cancels with `/iter-reset`

## Integration with Go Binary

The skill uses the compiled `iter` binary for:
- State management
- Hook handling
- Prompt generation
- Verdict recording

Build the binary:
```bash
go build -o bin/iter ./cmd/iter
```
