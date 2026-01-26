# iter-search Skill

## Overview

The `iter-search` skill searches your indexed codebase using semantic or keyword search. It returns relevant code locations with context, helping you quickly navigate and understand your code.

**Prerequisites**: The code index must be built first using `/iter:iter-index build`.

## Usage

```bash
/iter:iter-search "<query>"
```

### Examples

```bash
# Find authentication code
/iter:iter-search "user authentication"

# Find error handling
/iter:iter-search "error handling patterns"

# Find specific functionality
/iter:iter-search "database connection setup"

# Find API endpoints
/iter:iter-search "REST API endpoints"
```

## Prerequisites

Before using iter-search, you must build the code index:

```bash
# First time: build the index
/iter:iter-index build

# Check index status
/iter:iter-index status

# Then search
/iter:iter-search "<your query>"
```

If the index doesn't exist or is outdated, search results may be empty or incomplete.

## Search Modes

The skill automatically chooses the best search mode:

### Semantic Search
- **Understands meaning and context**
- Finds conceptually related code
- Works with natural language queries
- Example: "how users log in" finds authentication code

### Keyword Search
- **Exact or fuzzy text matching**
- Finds literal occurrences
- Works with specific identifiers
- Example: "loginUser" finds that function name

The skill intelligently selects the appropriate mode based on your query.

## How It Works

1. **Executes query** - Runs `iter search` binary command
2. **Queries the index** - Searches `.iter/index/` directory
3. **Ranks results** - Orders by relevance
4. **Returns matches** - File paths, line numbers, and context
5. **Claude presents** - Formats results with helpful context

## Output Format

Search results include:

- **File paths** - Where the code is located
- **Line numbers** - Exact locations in files
- **Code context** - Surrounding lines for understanding
- **Relevance ranking** - Best matches first

### Example Output

```
Found 3 matches for "user authentication":

1. src/auth/login.go:45
   Function: authenticateUser
   Validates user credentials and creates session

   Context:
   43: func authenticateUser(username, password string) (*User, error) {
   44:     // Authenticate user against database
   45:     user, err := db.FindUserByUsername(username)
   46:     if err != nil {
   47:         return nil, err
   48:     }

2. src/middleware/auth.go:23
   Middleware: requireAuth
   Checks if user is authenticated before allowing access

   Context:
   21: func requireAuth(next http.Handler) http.Handler {
   22:     return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
   23:         // Check authentication token
   24:         token := r.Header.Get("Authorization")

3. src/api/routes.go:12
   Route: POST /api/login
   Authentication endpoint

   Context:
   10: router.HandleFunc("/api/login", handlers.Login).Methods("POST")
   11: router.HandleFunc("/api/logout", handlers.Logout).Methods("POST")
   12: router.HandleFunc("/api/user", middleware.RequireAuth(handlers.GetUser))
```

## Usage Tips

### Effective Queries

**Good queries** (descriptive, natural language):
```bash
/iter:iter-search "how users authenticate"
/iter:iter-search "error handling for API requests"
/iter:iter-search "database connection pooling"
```

**Also good** (specific identifiers):
```bash
/iter:iter-search "authenticateUser function"
/iter:iter-search "DatabasePool class"
/iter:iter-search "API middleware"
```

### Try Different Phrasings

If you don't get good results, try rephrasing:

```bash
# Try these variations:
/iter:iter-search "user login"
/iter:iter-search "authentication flow"
/iter:iter-search "login handler"
```

### Rebuild Index if Code Changed

If you've made significant code changes:

```bash
# Rebuild index for fresh results
/iter:iter-index build

# Then search again
/iter:iter-search "<query>"
```

### Use Grep for Exact Patterns

For literal pattern matching, use Grep instead:

```bash
# iter-search: semantic/fuzzy
/iter:iter-search "authentication"

# Grep: exact literal match
# Use Grep tool with pattern: "func authenticate"
```

## When to Use

**Use iter-search when:**
- Exploring unfamiliar code
- Finding all instances of a concept
- Understanding architecture
- Locating relevant functions/classes
- Discovering patterns in codebase

**Use Grep instead when:**
- You know the exact text to find
- You need regex pattern matching
- You want literal string search
- You need to search in specific files only

## Examples

### Example 1: Find Authentication Code

```bash
User: /iter:iter-search "user authentication"

Claude: Found 5 matches:

        1. src/auth/login.go:45 - authenticateUser function
        2. src/middleware/auth.go:23 - requireAuth middleware
        3. src/api/handlers/login.go:18 - Login HTTP handler
        4. tests/auth_test.go:34 - authentication test cases
        5. docs/API.md:12 - authentication documentation

        Would you like me to read any of these files for more details?
```

### Example 2: Find Error Handling

```bash
User: /iter:iter-search "error handling patterns"

Claude: Found 8 matches:

        1. src/errors/handler.go - Central error handling
        2. src/middleware/errors.go - Error middleware
        3. src/api/errors.go - API error responses
        ...

        The codebase uses a centralized error handling pattern.
        Main handler is in src/errors/handler.go:12.

        Shall I read that file to explain the pattern?
```

### Example 3: Understand Database Layer

```bash
User: /iter:iter-search "database connection and queries"

Claude: Found 6 matches:

        1. src/db/connection.go - Database connection pool
        2. src/db/queries.go - Query builder
        3. src/repositories/user.go - User repository with DB queries
        ...

        The database layer uses repository pattern:
        - Connection pooling in src/db/connection.go
        - Query builder in src/db/queries.go
        - Repositories in src/repositories/*

        Want me to explain the architecture?
```

## Related Skills

- **/iter:iter-index** - Build the search index (required before searching)
- **Grep** - Exact literal pattern matching (doesn't require index)
- **Glob** - Find files by name patterns

## Typical Workflow

1. **Build index** (first time or after major changes):
   ```bash
   /iter:iter-index build
   ```

2. **Search for code**:
   ```bash
   /iter:iter-search "authentication logic"
   ```

3. **Read relevant files**:
   ```bash
   # Ask Claude to read specific files from results
   "Can you read src/auth/login.go and explain how authentication works?"
   ```

4. **Make changes** and update index:
   ```bash
   # After code changes
   /iter:iter-index build
   ```

## Troubleshooting

### No Results Found

1. **Check index exists**:
   ```bash
   /iter:iter-index status
   ```

2. **Build index if missing**:
   ```bash
   /iter:iter-index build
   ```

3. **Try different search terms**:
   - Rephrase your query
   - Try more specific or more general terms
   - Use specific function/class names

### Outdated Results

If results don't match your current code:

```bash
# Rebuild index
/iter:iter-index build

# Search again
/iter:iter-search "<query>"
```

### Index Not Built

```
Error: Code index not found

Solution:
1. Run: /iter:iter-index build
2. Wait for indexing to complete
3. Try your search again
```

## Performance

- **Query time**: Milliseconds (very fast)
- **Result limit**: Top 10-20 most relevant matches
- **Context lines**: 5-10 lines around each match
- **Relevance ranking**: Best matches shown first

## Technical Notes

- Uses the index built by `/iter:iter-index`
- Combines semantic understanding with keyword matching
- Respects `.gitignore` patterns (doesn't search excluded files)
- Index is local (not shared across machines)
- Safe to use concurrently with other operations
- Results are read-only (doesn't modify code)

## Advanced Usage

### Combine with Other Skills

```bash
# Search for code
/iter:iter-search "authentication"

# Then use iter to implement changes
/iter "add two-factor authentication support"

# Or run tests
/iter-test tests/auth_test.go
```

### Search-Driven Development

1. Search to understand existing patterns:
   ```bash
   /iter:iter-search "how API endpoints are defined"
   ```

2. Use findings to guide new implementation:
   ```bash
   /iter "add new /api/users endpoint following existing pattern"
   ```

3. Update index with new code:
   ```bash
   /iter:iter-index build
   ```
