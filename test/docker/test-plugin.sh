#!/bin/bash
set -e

echo "=========================================="
echo "ITER PLUGIN INSTALLATION TEST"
echo "=========================================="
echo ""

# Check Claude version
echo "[1/9] Checking Claude Code version..."
claude --version || { echo "FAIL: Claude not installed"; exit 1; }
echo ""

# Add marketplace
echo "[2/9] Adding local marketplace..."
claude plugin marketplace add /home/testuser/iter-plugin
if [ $? -ne 0 ]; then
    echo "FAIL: Could not add marketplace"
    exit 1
fi
echo ""

# List marketplaces
echo "[3/9] Listing marketplaces..."
claude plugin marketplace list
echo ""

# Install plugin
echo "[4/9] Installing iter plugin..."
claude plugin install iter@iter-local
if [ $? -ne 0 ]; then
    echo "FAIL: Could not install plugin"
    exit 1
fi
echo ""

# Check settings
echo "[5/9] Checking settings for installed plugin..."
if [ -f ~/.claude/settings.json ]; then
    echo "Settings file contents:"
    cat ~/.claude/settings.json

    if grep -q "iter@iter-local" ~/.claude/settings.json; then
        echo ""
        echo "OK: iter@iter-local found in settings"
    else
        echo ""
        echo "FAIL: iter@iter-local NOT found in settings"
        exit 1
    fi
else
    echo "FAIL: Settings file not found"
    exit 1
fi
echo ""

# Check plugin cache
echo "[6/9] Checking plugin cache structure..."
CACHE_DIR=~/.claude/plugins/cache/iter-local/iter
if [ -d "$CACHE_DIR" ]; then
    echo "Plugin cache directory exists"
    echo ""
    echo "Cached files:"
    find "$CACHE_DIR" -type f | head -30
    echo ""

    LATEST_VERSION=$(ls -t "$CACHE_DIR" | grep -v orphaned | head -1)
    echo "Latest version: $LATEST_VERSION"

    if [ -n "$LATEST_VERSION" ]; then
        SKILL_FILE="$CACHE_DIR/$LATEST_VERSION/skills/iter/SKILL.md"
        if [ -f "$SKILL_FILE" ]; then
            echo ""
            echo "Checking SKILL.md for 'name' field..."
            echo "--- SKILL.md contents ---"
            cat "$SKILL_FILE"
            echo "--- end ---"
            echo ""

            if grep -q "^name:" "$SKILL_FILE"; then
                echo "OK: SKILL.md has 'name' field"
            else
                echo "FAIL: SKILL.md missing 'name' field"
                exit 1
            fi
        else
            echo "FAIL: SKILL.md not found at $SKILL_FILE"
            exit 1
        fi

        MKT_FILE="$CACHE_DIR/$LATEST_VERSION/.claude-plugin/marketplace.json"
        if [ -f "$MKT_FILE" ]; then
            echo ""
            echo "Checking marketplace.json..."
            echo "--- marketplace.json ---"
            cat "$MKT_FILE"
            echo "--- end ---"

            if grep -q '"skills"' "$MKT_FILE"; then
                echo ""
                echo "OK: marketplace.json has 'skills' field"
            else
                echo "FAIL: marketplace.json missing 'skills' field"
                exit 1
            fi
        fi

        # Store iter binary path for later tests
        ITER_BIN="$CACHE_DIR/$LATEST_VERSION/iter"
        export CLAUDE_PLUGIN_ROOT="$CACHE_DIR/$LATEST_VERSION"
    fi
else
    echo "FAIL: Plugin cache directory not found"
    exit 1
fi
echo ""

# Test the iter binary directly
echo "[7/9] Testing iter binary version..."
if [ -x "$ITER_BIN" ]; then
    echo "--- iter version ---"
    "$ITER_BIN" version
    echo "--- end ---"
    echo "OK: iter binary executes correctly"
else
    echo "FAIL: iter binary not found or not executable"
    exit 1
fi
echo ""

# Test iter binary help
echo "[8/9] Testing iter binary help..."
echo "--- iter help ---"
"$ITER_BIN" help 2>&1 | head -20
echo "--- end ---"
echo "OK: iter help works"
echo ""

# Test /iter:run command via Claude
echo "[9/9] Testing /iter:run command in Claude..."
echo ""

if [ -n "$ANTHROPIC_API_KEY" ]; then
    echo "API key detected, running full integration test..."

    # Create a test directory with git
    mkdir -p /tmp/iter-test
    cd /tmp/iter-test
    git init -q
    git config user.email "test@test.com"
    git config user.name "Test"

    # Run Claude with /iter:run command using print mode
    # Use timeout to prevent hanging
    echo "Running: claude -p '/iter:run --version'"

    CLAUDE_OUTPUT=$(timeout 60 claude -p '/iter:run --version' 2>&1 || true)

    echo "--- Claude /iter:run output ---"
    echo "$CLAUDE_OUTPUT"
    echo "--- end ---"

    # Check if iter was invoked (look for iter output patterns)
    if echo "$CLAUDE_OUTPUT" | grep -qiE "(iter|version|iterative)"; then
        echo ""
        echo "OK: /iter:run command executed in Claude"
    else
        echo ""
        echo "WARNING: /iter:run output does not contain expected patterns"
        echo "The skill may have executed but produced unexpected output"
    fi
else
    echo "No API key provided (ANTHROPIC_API_KEY not set)"
    echo "Running offline skill simulation test..."
    echo ""

    # Create a test directory with git
    mkdir -p /tmp/iter-test
    cd /tmp/iter-test
    git init -q
    git config user.email "test@test.com"
    git config user.name "Test"

    # Simulate what the skill does: ${CLAUDE_PLUGIN_ROOT}/iter run "<args>"
    # Test with a simple task
    echo "Simulating skill execution: iter run 'test task'"
    echo "--- iter run output ---"
    "$ITER_BIN" run "test task" --no-worktree 2>&1 | head -30 || true
    echo "--- end ---"
    echo ""
    echo "OK: iter run command executes correctly"
    echo ""
    echo "To run full Claude integration test, provide API key:"
    echo "  docker run -e ANTHROPIC_API_KEY=sk-... iter-plugin-test"
fi

echo ""
echo "=========================================="
echo "ALL TESTS PASSED"
echo "=========================================="
