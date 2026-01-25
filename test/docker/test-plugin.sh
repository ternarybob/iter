#!/bin/bash
set -e

echo "=========================================="
echo "ITER PLUGIN INSTALLATION TEST"
echo "=========================================="
echo ""

# Check Claude version
echo "[1/10] Checking Claude Code version..."
claude --version || { echo "FAIL: Claude not installed"; exit 1; }
echo ""

# Add marketplace
echo "[2/10] Adding local marketplace..."
claude plugin marketplace add /home/testuser/iter-plugin
if [ $? -ne 0 ]; then
    echo "FAIL: Could not add marketplace"
    exit 1
fi
echo ""

# List marketplaces
echo "[3/10] Listing marketplaces..."
claude plugin marketplace list
echo ""

# Install plugin
echo "[4/10] Installing iter plugin..."
claude plugin install iter@iter-local
if [ $? -ne 0 ]; then
    echo "FAIL: Could not install plugin"
    exit 1
fi
echo ""

# Check settings
echo "[5/10] Checking settings for installed plugin..."
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
echo "[6/10] Checking plugin cache structure..."
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
echo "[7/10] Testing iter binary version..."
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
echo "[8/10] Testing iter binary help..."
echo "--- iter help ---"
"$ITER_BIN" help 2>&1 | head -20
echo "--- end ---"
echo "OK: iter help works"
echo ""

# Require API key for Claude integration tests
if [ -z "$ANTHROPIC_API_KEY" ]; then
    echo "=========================================="
    echo "FAIL: ANTHROPIC_API_KEY not set"
    echo "=========================================="
    echo ""
    echo "Claude integration tests require an API key."
    echo "Set ANTHROPIC_API_KEY in test/docker/.env or pass via:"
    echo "  docker run -e ANTHROPIC_API_KEY=sk-... iter-plugin-test"
    echo ""
    exit 1
fi

# Create a test directory with git
mkdir -p /tmp/iter-test
cd /tmp/iter-test
git init -q
git config user.email "test@test.com"
git config user.name "Test"

# Test 1: claude -p "/iter:run -v" (command line invocation)
echo "[9/10] Testing: claude -p '/iter:run -v' (command line)..."
echo ""

CLAUDE_CMD_OUTPUT=$(timeout 120 claude -p '/iter:run -v' 2>&1) || CMD_EXIT=$?
CMD_EXIT=${CMD_EXIT:-0}

echo "--- claude -p '/iter:run -v' output ---"
echo "$CLAUDE_CMD_OUTPUT"
echo "--- end (exit code: $CMD_EXIT) ---"
echo ""

# Check if /iter:run was recognized and executed
if echo "$CLAUDE_CMD_OUTPUT" | grep -qiE "(iter version|ITERATIVE IMPLEMENTATION)"; then
    echo "OK: /iter:run -v executed via command line"
    CMD_TEST_PASS=1
else
    echo "FAIL: /iter:run -v did NOT execute properly via command line"
    echo "Expected output to contain 'iter version' or 'ITERATIVE IMPLEMENTATION'"
    CMD_TEST_PASS=0
fi
echo ""

# Test 2: Interactive mode - send /iter:run -v to running claude
echo "[10/10] Testing: /iter:run -v in interactive Claude session..."
echo ""

# Use expect-like approach with timeout and stdin
INTERACTIVE_OUTPUT=$(timeout 120 bash -c '
echo "/iter:run -v" | claude --dangerously-skip-permissions 2>&1
' 2>&1) || INT_EXIT=$?
INT_EXIT=${INT_EXIT:-0}

echo "--- Interactive /iter:run -v output ---"
echo "$INTERACTIVE_OUTPUT"
echo "--- end (exit code: $INT_EXIT) ---"
echo ""

# Check if /iter:run was recognized and executed
if echo "$INTERACTIVE_OUTPUT" | grep -qiE "(iter version|ITERATIVE IMPLEMENTATION)"; then
    echo "OK: /iter:run -v executed in interactive mode"
    INT_TEST_PASS=1
else
    echo "FAIL: /iter:run -v did NOT execute properly in interactive mode"
    echo "Expected output to contain 'iter version' or 'ITERATIVE IMPLEMENTATION'"
    INT_TEST_PASS=0
fi
echo ""

# Final result
echo "=========================================="
echo "TEST RESULTS"
echo "=========================================="
echo "Command line test (claude -p '/iter:run -v'): $([ $CMD_TEST_PASS -eq 1 ] && echo 'PASS' || echo 'FAIL')"
echo "Interactive test (/iter:run -v in session):  $([ $INT_TEST_PASS -eq 1 ] && echo 'PASS' || echo 'FAIL')"
echo ""

if [ $CMD_TEST_PASS -eq 1 ] && [ $INT_TEST_PASS -eq 1 ]; then
    echo "=========================================="
    echo "ALL TESTS PASSED"
    echo "=========================================="
    exit 0
else
    echo "=========================================="
    echo "TESTS FAILED"
    echo "=========================================="
    echo ""
    echo "The /iter:run command is not being recognized or executed by Claude."
    echo "Check that:"
    echo "  1. The plugin is properly installed"
    echo "  2. Skills are correctly defined in marketplace.json"
    echo "  3. SKILL.md files have the 'name' field"
    echo ""
    exit 1
fi
