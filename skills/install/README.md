# install Skill

## Overview

The `install` skill creates a convenient wrapper that allows you to use `/iter` instead of `/iter:run`. This is a one-time setup that creates a shortcut skill in your Claude Code user skills directory.

After installation, you can use the shorter `/iter` command directly in Claude Code sessions.

## Usage

```bash
/iter:install
```

That's it! The skill will detect your operating system and create the appropriate wrapper automatically.

## How It Works

The install skill:

1. **Detects your OS** (Linux, macOS, WSL, or Windows)
2. **Creates an installation script** in `.iter/` directory:
   - `install-skill.sh` for Unix-like systems (Linux/macOS/WSL)
   - `install-skill.ps1` for Windows
3. **Executes the script** to create the wrapper
4. **Creates wrapper SKILL.md** in `~/.claude/skills/iter/`

The wrapper skill is simple - it delegates all arguments to the plugin's `/iter:run` command:

```markdown
---
name: iter
description: Run iter default workflow (wrapper for iter plugin)
---

Execute the plugin skill `/iter:run` with the same arguments.

Arguments:
$ARGUMENTS
```

## Post-Installation

After running `/iter:install`:

1. **Restart Claude Code** to load the new wrapper skill
2. **Use the `/iter` command** directly in your sessions

### Before Installation
```bash
/iter:run "add health check endpoint"
```

### After Installation
```bash
/iter "add health check endpoint"
```

Much shorter and more convenient!

## Platform Support

✅ **Linux** - Creates bash script, installs to `~/.claude/skills/iter`
✅ **macOS** - Creates bash script, installs to `~/.claude/skills/iter`
✅ **WSL** - Creates bash script, installs to `~/.claude/skills/iter`
✅ **Windows** - Creates PowerShell script, installs to `%USERPROFILE%\.claude\skills\iter`

The skill automatically detects your platform and creates the appropriate installer.

## Example Session

```
User: /iter:install

Claude: Installing iter wrapper skill...

        Created installation script: .iter/install-skill.sh
        Executing installer...

        ✅ iter skill installed successfully

        Restart Claude Code and use:

          /iter "<task description>"

        Example: /iter "add health check endpoint"
```

## Wrapper Location

The wrapper skill is installed to:

- **Unix-like** (Linux/macOS/WSL): `~/.claude/skills/iter/SKILL.md`
- **Windows**: `%USERPROFILE%\.claude\skills\iter\SKILL.md`

This location is in your user-level skills directory, so the wrapper is available in all Claude Code sessions.

## Uninstalling the Wrapper

To remove the `/iter` wrapper:

**Unix-like systems:**
```bash
rm -rf ~/.claude/skills/iter
```

**Windows:**
```powershell
Remove-Item -Recurse -Force "$env:USERPROFILE\.claude\skills\iter"
```

Then restart Claude Code. You can still use `/iter:run` from the plugin.

## Related Skills

- **/iter:run** - The main iterative implementation skill (what the wrapper delegates to)
- **/iter:iter-workflow** - Custom workflow-based implementation
- **/iter:iter-test** - Test-driven iteration with auto-fix

## Technical Notes

- The wrapper is a user-level skill (not project-specific)
- It simply delegates to the plugin's `/iter:run` command
- No functionality changes - just a shorter command name
- Can be reinstalled anytime by running `/iter:install` again
- Safe to run multiple times (overwrites existing wrapper)
