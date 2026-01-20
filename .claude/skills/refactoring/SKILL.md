# Refactoring Skill

## Purpose
Core patterns for ALL code modifications in Quaero. Referenced by commands and other skills.

## FUNDAMENTAL RULES
```
┌─────────────────────────────────────────────────────────────────┐
│ ANTI-CREATION BIAS                                              │
│                                                                  │
│ PRIORITY: EXTEND > MODIFY > CREATE                              │
│                                                                  │
│ Before creating ANYTHING, prove existing code can't be extended │
│ CREATE requires written justification:                          │
│ • Why existing code cannot be extended                          │
│ • What pattern from existing codebase it follows                │
└─────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│ BACKWARD COMPATIBILITY IS NOT REQUIRED                          │
│                                                                  │
│ • Requirements define the ONLY valid behavior                   │
│ • Legacy code behavior is IRRELEVANT if requirements differ     │
│ • Breaking changes are ALLOWED to satisfy requirements          │
│ • NEVER add compatibility shims for old behavior                │
└─────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│ CLEANUP IS MANDATORY                                            │
│                                                                  │
│ • Remove redundant functions when replacing with new impl       │
│ • Delete dead code paths made obsolete by changes               │
│ • Remove unused parameters, variables, and imports              │
│ • Consolidate duplicate logic - don't leave old version         │
│                                                                  │
│ Leave the codebase CLEANER than you found it.                   │
└─────────────────────────────────────────────────────────────────┘
```

## BUILD REQUIREMENT
```
OS Detection:
- C:\... or D:\... = Windows
- /home/... or /Users/... = Unix/Linux/macOS

Build Commands:
- Windows:     .\scripts\build.ps1
- Linux/macOS: ./scripts/build.sh

BUILD FAIL = TASK FAIL (no exceptions)
```

## ARCHITECTURE COMPLIANCE

Read before modifying code:
- `docs/architecture/*.md` - Architecture requirements (LAW)
- `.claude/skills/go/SKILL.md` - Go patterns
- `.claude/skills/frontend/SKILL.md` - Frontend patterns

## CODE MODIFICATION RULES

### Before ANY Change:
1. Search codebase for existing similar code
2. Identify extension points (interfaces, services, patterns)
3. Challenge: Does this NEED new code?

### When Modifying:
1. Follow EXACT patterns from existing codebase
2. Minimum viable change (not "proper" or "complete")
3. Remove any code made redundant by changes
4. Build must pass before completion

### Forbidden:
- Creating parallel structures
- Duplicating existing logic
- Ignoring existing patterns
- Modifying tests to make code pass
- Leaving dead code behind
- Adding backward compatibility shims