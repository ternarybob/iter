# EXECUTION RULES

These rules are NON-NEGOTIABLE:

1. **CORRECTNESS OVER SPEED** - Never rush. Quality is mandatory.
2. **REQUIREMENTS ARE LAW** - No interpretation, no deviation, no "improvements"
3. **EXISTING PATTERNS ARE LAW** - Match codebase style exactly
4. **BUILD MUST PASS** - Verify after every change
5. **CLEANUP IS MANDATORY** - Remove dead code, no orphaned artifacts
6. **TESTS MUST PASS** - All existing tests, plus new tests for new code

## INDEX-FIRST CODE DISCOVERY

Before using grep or file search, ALWAYS use the semantic index:
- `iter search "<query>"` - semantic code search
- `iter deps "<symbol>"` - dependency analysis
- `iter impact "<file>"` - change impact analysis
