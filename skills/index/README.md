# index Skill

## Overview

The `index` skill manages the code index used for semantic and keyword search. It provides commands to build, monitor, and maintain a searchable index of your codebase.

The index enables fast code navigation and discovery through the `/iter:search` skill.

## Commands

| Command | Description |
|---------|-------------|
| `status` | Show current index state and statistics |
| `build` | Create or rebuild the code index |
| `clear` | Remove all index data |
| `watch` | Monitor codebase and auto-update index |

## Usage

```bash
# Check index status
/iter:index status

# Build or rebuild index
/iter:index build

# Clear index data
/iter:index clear

# Start watch mode (auto-update)
/iter:index watch
```

## How It Works

The skill executes the `iter index` binary command, which:

1. **Scans the codebase** - Walks through all source files
2. **Parses code structure** - Understands functions, classes, imports
3. **Creates searchable index** - Stores in `.iter/index/` directory
4. **Enables fast queries** - Powers `/iter:search` skill

The index is stored locally in your project's `.iter/index/` directory.

## When to Use

### Build the Index

Run `/iter:index build` when:
- First time setting up search functionality
- After significant code changes
- When search results seem outdated
- After pulling major changes from git

### Check Status

Run `/iter:index status` to:
- Verify index exists
- See how many files are indexed
- Check index size and timestamp
- Confirm index is up-to-date

### Clear the Index

Run `/iter:index clear` to:
- Remove stale index data
- Free up disk space
- Start fresh before rebuilding

### Watch Mode

Run `/iter:index watch` to:
- Automatically update index when files change
- Keep index fresh during active development
- Avoid manual rebuilds

## Examples

### Example 1: First-Time Setup

```bash
User: /iter:index build

Claude: Building code index...

        Scanning codebase...
        - Found 342 source files
        - Indexed 15,234 code entities
        - Size: 2.4 MB

        ✅ Index built successfully

        Location: .iter/index/

        You can now use /iter:search to query the codebase!
```

### Example 2: Check Index Status

```bash
User: /iter:index status

Claude: Code Index Status

        Status: READY
        Files indexed: 342
        Code entities: 15,234
        Index size: 2.4 MB
        Last updated: 2026-01-26 14:23:15
        Location: .iter/index/

        Index is up-to-date and ready for searching.
```

### Example 3: Clear and Rebuild

```bash
User: /iter:index clear

Claude: ✅ Index cleared successfully

        Removed: .iter/index/

User: /iter:index build

Claude: Building code index...
        ✅ Index built successfully
```

## Output

### Index Location
`.iter/index/` - All index data stored here

### Statistics Reported
- Number of files indexed
- Total code entities (functions, classes, etc.)
- Index size on disk
- Last update timestamp
- Index status (READY, BUILDING, MISSING)

## Index Structure

The `.iter/index/` directory contains:
- **Metadata** - File paths, timestamps, structure
- **Code entities** - Functions, classes, imports, exports
- **Search data** - Optimized for fast queries
- **Embeddings** - For semantic search (if enabled)

You don't need to interact with these files directly - the skills handle all index operations.

## Performance

- **Build time**: Depends on codebase size (typically seconds to minutes)
- **Index size**: ~1-5 MB per 100 source files
- **Query time**: Milliseconds for most searches
- **Watch overhead**: Minimal (only indexes changed files)

## Related Skills

- **/iter:search** - Search the indexed code (requires index to be built)
- **Grep** - For exact literal pattern matching (doesn't require index)

## Typical Workflow

1. **Build index once**:
   ```bash
   /iter:index build
   ```

2. **Search as needed**:
   ```bash
   /iter:search "authentication logic"
   /iter:search "error handling patterns"
   ```

3. **Rebuild periodically**:
   ```bash
   # After major changes
   /iter:index build
   ```

4. **Or use watch mode**:
   ```bash
   # During active development
   /iter:index watch
   ```

## Troubleshooting

### "Index not found"
Run `/iter:index build` to create the index.

### "Index seems outdated"
Run `/iter:index build` to rebuild with latest code.

### Search returns no results
1. Check index exists: `/iter:index status`
2. Rebuild if needed: `/iter:index build`
3. Try different search terms

### Index taking too long to build
- Large codebases take longer (normal)
- Can interrupt and restart anytime
- Consider using `.gitignore` patterns to exclude generated code

## Technical Notes

- Index respects `.gitignore` patterns
- Supports all major programming languages
- Incremental updates in watch mode
- Safe to delete `.iter/index/` anytime (just rebuild)
- Index is local (not committed to git)
