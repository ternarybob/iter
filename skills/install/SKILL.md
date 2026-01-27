---
name: install
description: Install /iter shortcut skill. Creates a wrapper in ~/.claude/skills/iter so you can use /iter instead of /iter:iter. Supports Linux, macOS, WSL, and Windows.
allowed-tools: ["Bash", "Write"]
---

Install the iter shortcut skill by creating a wrapper in the user's Claude skills directory.

## Instructions

Detect the OS and create the appropriate installer file, then run it:

### For Linux/macOS/WSL (paths starting with `/`):

Create file `.iter/install-skill.sh`:

```bash
#!/usr/bin/env bash
set -e

CLAUDE_DIR="$HOME/.claude/skills/iter"

echo "Installing iter wrapper skill..."

mkdir -p "$CLAUDE_DIR"

cat << 'SKILLEOF' > "$CLAUDE_DIR/SKILL.md"
---
name: iter
description: Adversarial iterative implementation. Use -v for version, -t:<file> for test mode, -w:<file> for workflow mode, -r to reindex, or just provide a task description.
---

Execute the plugin skill `/iter:iter` with the same arguments.

Arguments:
$ARGUMENTS
SKILLEOF

echo ""
echo "iter skill installed successfully"
echo ""
echo "Restart Claude Code and use:"
echo ""
echo "  /iter"
echo ""
```

Then run: `bash .iter/install-skill.sh`

### For Windows (paths starting with `C:\` or similar drive letter):

Create file `.iter/install-skill.ps1`:

```powershell
$ClaudeDir = "$env:USERPROFILE\.claude\skills\iter"

Write-Host "Installing iter wrapper skill..."

New-Item -ItemType Directory -Force -Path $ClaudeDir | Out-Null

$skill = @'
---
name: iter
description: Adversarial iterative implementation. Use -v for version, -t:<file> for test mode, -w:<file> for workflow mode, -r to reindex, or just provide a task description.
---

Execute the plugin skill `/iter:iter` with the same arguments.

Arguments:
$ARGUMENTS
'@

$skill | Out-File -Encoding utf8 "$ClaudeDir\SKILL.md"

Write-Host ""
Write-Host "iter skill installed successfully"
Write-Host ""
Write-Host "Restart Claude Code and use:"
Write-Host ""
Write-Host "  /iter"
Write-Host ""
```

Then run: `powershell -ExecutionPolicy Bypass -File .iter/install-skill.ps1`

## After Installation

Tell the user to restart Claude Code to use `/iter`.
