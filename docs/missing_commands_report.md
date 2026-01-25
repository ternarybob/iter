# Plugin Configuration Diagnosis

## Issue
The user reported that `/iter` commands are not showing up in the Claude CLI, despite `skills/` being implemented.

## Investigation Findings

1.  **Skills Implementation is Correct**:
    - The project uses `skills/` to define commands (e.g., `skills/iter/SKILL.md`), which is a valid alternative to `commands/`.
    - The `SKILL.md` file correctly attempts to invoke the binary: `!${CLAUDE_PLUGIN_ROOT}/iter`.

2.  **Structural Mismatch in Build Output**:
    - `scripts/build.sh` places `plugin.json` in `bin/.claude-plugin/plugin.json`.
    - `scripts/build.sh` places skills in `bin/skills/`.
    - `plugin.json` defines `"skills": "./skills/"`.

3.  **Path Resolution Failure**:
    - Relative to `bin/.claude-plugin/plugin.json`, Claude Code looks for skills in `bin/.claude-plugin/skills/`.
    - Since skills are actually in `bin/skills/`, **Claude Code fails to find any skills**.

4.  **Binary Location Mismatch**:
    - Even if skills were found, `SKILL.md` uses `${CLAUDE_PLUGIN_ROOT}/iter`.
    - If `plugin.json` is in `bin/.claude-plugin/`, that directory is considered the plugin root.
    - Therefore, it would look for the binary at `bin/.claude-plugin/iter`.
    - The binary is actually at `bin/iter`.
    - **Command execution would fail** even if the command was registered.

## Conclusion
The build script incorrectly nests `plugin.json` inside `.claude-plugin`. For the relative paths in `plugin.json` and `SKILL.md` to work, `plugin.json` must be in the same directory as the `iter` binary and the `skills/` folder.

## Recommendations

### 1. Update `scripts/build.sh`
Modify the build script to place `plugin.json` directly in the `bin/` root.

**Change:**
```bash
cp "$PROJECT_DIR/config/plugin.json" "$PROJECT_DIR/bin/.claude-plugin/plugin.json"
```
**To:**
```bash
cp "$PROJECT_DIR/config/plugin.json" "$PROJECT_DIR/bin/plugin.json"
```

### 2. Verify
1. Run `./scripts/build.sh`.
2. Confirm `bin/plugin.json` exists.
3. Confirm `bin/skills/` exists.
4. Confirm `bin/iter` exists.

This restores the correct relationship:
- Root: `bin/`
- Skills: `bin/skills/` (found via `./skills/`)
- Binary: `bin/iter` (found via `${CLAUDE_PLUGIN_ROOT}/iter`)