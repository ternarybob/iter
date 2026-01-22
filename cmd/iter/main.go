// Package main provides the CLI entry point for the iter Claude Code plugin.
//
// iter is an adversarial multi-agent DevOps loop that runs within Claude Code.
// It implements a structured iterative approach where work is planned, executed,
// and validated until requirements/tests are achieved.
//
// Usage:
//
//	iter run "<task>"                  - Start iterative implementation
//	iter workflow "<spec>"             - Start workflow-based implementation
//	iter status                        - Show current session status
//	iter pass                          - Record validation pass
//	iter reject "<reason>"             - Record validation rejection
//	iter next                          - Move to next step
//	iter complete                      - Mark session complete
//	iter reset                         - Reset session state
//	iter hook-stop                     - Stop hook handler (JSON output)
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/ternarybob/iter/index"
)

// OSInfo contains detected operating system information.
type OSInfo struct {
	OS        string // "linux", "darwin", "windows"
	IsWSL     bool   // Running in Windows Subsystem for Linux
	WSLDistro string // WSL distribution name if applicable
}

// detectOS detects the operating system and WSL environment.
func detectOS() OSInfo {
	info := OSInfo{
		OS: runtime.GOOS,
	}

	// Detect WSL
	if runtime.GOOS == "linux" {
		// Check for WSL indicators
		if data, err := os.ReadFile("/proc/version"); err == nil {
			version := strings.ToLower(string(data))
			if strings.Contains(version, "microsoft") || strings.Contains(version, "wsl") {
				info.IsWSL = true
				// Try to get WSL distro name
				if distro := os.Getenv("WSL_DISTRO_NAME"); distro != "" {
					info.WSLDistro = distro
				}
			}
		}
		// Also check for WSL interop
		if _, err := os.Stat("/proc/sys/fs/binfmt_misc/WSLInterop"); err == nil {
			info.IsWSL = true
		}
	}

	return info
}

// getOSGuidance returns OS-specific guidance for Claude.
func getOSGuidance(info OSInfo) string {
	var guidance strings.Builder

	guidance.WriteString("## ENVIRONMENT\n\n")
	guidance.WriteString(fmt.Sprintf("- **OS**: %s\n", info.OS))

	if info.IsWSL {
		guidance.WriteString(fmt.Sprintf("- **WSL**: Yes (distro: %s)\n", info.WSLDistro))
		guidance.WriteString(`
### WSL-SPECIFIC RULES (CRITICAL)

You are running in Windows Subsystem for Linux. Follow these rules:

1. **NEVER use /mnt/c paths for git operations** - Cross-filesystem operations are unreliable
2. **For directory renames**: Use standard 'mv' or 'git mv' - if it fails with "Invalid argument", this is a WSL/NTFS limitation
3. **If filesystem operations fail 2 times**: STOP and report the blocker to the user
4. **Do NOT try workarounds** like cmd.exe, xcopy, or Windows commands - they create metadata inconsistencies
5. **Preferred approach for cross-filesystem issues**:
   - Ask user to run the operation from Windows side, OR
   - Work entirely within the Linux filesystem (/home/...)

### BLOCKER HANDLING

If you encounter:
- "Invalid argument" on rename/move
- "No such file or directory" for files that exist on Windows
- Directory metadata inconsistencies

Then STOP immediately and report:
1. The specific error
2. That this is a WSL filesystem limitation
3. Ask the user how they want to proceed
`)
	} else if info.OS == "darwin" {
		guidance.WriteString("- **Platform**: macOS\n")
		guidance.WriteString(`
### macOS-SPECIFIC NOTES

- Use standard Unix commands for file operations
- Case-insensitive filesystem by default (APFS)
- Homebrew paths may be in /opt/homebrew (Apple Silicon) or /usr/local (Intel)
`)
	} else if info.OS == "windows" {
		guidance.WriteString("- **Platform**: Windows (native)\n")
		guidance.WriteString(`
### WINDOWS-SPECIFIC NOTES

- Use PowerShell or cmd syntax for shell commands
- Path separators are backslashes (\) but forward slashes often work
- Be aware of path length limits (260 chars by default)
`)
	} else {
		guidance.WriteString("- **Platform**: Linux\n")
		guidance.WriteString(`
### LINUX NOTES

- Standard Unix commands available
- Case-sensitive filesystem
`)
	}

	guidance.WriteString(`
### GENERAL BLOCKER RULES

After **3 failed attempts** at the same operation:
1. STOP trying workarounds
2. Document what you tried
3. Report the blocker to the user
4. Ask for guidance before proceeding

Do NOT enter retry loops for environmental issues.
`)

	return guidance.String()
}

// Git worktree management

// gitCmd runs a git command and returns its output.
func gitCmd(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	output, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(output)), err
}

// getCurrentBranch returns the current git branch name.
func getCurrentBranch() (string, error) {
	return gitCmd("rev-parse", "--abbrev-ref", "HEAD")
}

// isGitRepo checks if the current directory is a git repository.
func isGitRepo() bool {
	_, err := gitCmd("rev-parse", "--git-dir")
	return err == nil
}

// hasUncommittedChanges checks if there are uncommitted changes.
func hasUncommittedChanges() bool {
	output, _ := gitCmd("status", "--porcelain")
	return output != ""
}

// createWorktree creates a git worktree for isolated work.
// Returns the worktree path and branch name.
func createWorktree(taskSlug string) (worktreePath, branchName string, err error) {
	// Get current branch
	currentBranch, err := getCurrentBranch()
	if err != nil {
		return "", "", fmt.Errorf("get current branch: %w", err)
	}

	// Get project root for .iter directory (not cwd)
	projectRoot := findProjectRoot()

	// Generate unique branch name
	timestamp := time.Now().Format("20060102-150405")
	branchName = fmt.Sprintf("iter/%s-%s", taskSlug, timestamp)

	// Worktree path in .iter/worktrees/ - use absolute path from project root
	worktreePath = filepath.Join(projectRoot, stateDir, "worktrees", branchName)

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(worktreePath), 0755); err != nil {
		return "", "", fmt.Errorf("create worktree parent dir: %w", err)
	}

	// Create the worktree with a new branch
	_, err = gitCmd("worktree", "add", "-b", branchName, worktreePath, currentBranch)
	if err != nil {
		return "", "", fmt.Errorf("create worktree: %w", err)
	}

	return worktreePath, branchName, nil
}

// mergeWorktree merges the worktree branch back to the original branch.
func mergeWorktree(state *State) error {
	if state.WorktreeBranch == "" || state.OriginalBranch == "" {
		return nil // No worktree to merge
	}

	// Change to original workdir
	originalWd, _ := os.Getwd()
	if state.OriginalWorkdir != "" {
		if err := os.Chdir(state.OriginalWorkdir); err != nil {
			return fmt.Errorf("change to original dir: %w", err)
		}
		defer func() { _ = os.Chdir(originalWd) }()
	}

	// Check for uncommitted changes in worktree first
	if state.WorktreePath != "" {
		if err := os.Chdir(state.WorktreePath); err == nil {
			if hasUncommittedChanges() {
				// Auto-commit any pending changes
				_, _ = gitCmd("add", "-A")
				_, _ = gitCmd("commit", "-m", fmt.Sprintf("iter: auto-commit before merge for task: %s", state.Task))
			}
			_ = os.Chdir(originalWd)
			if state.OriginalWorkdir != "" {
				_ = os.Chdir(state.OriginalWorkdir)
			}
		}
	}

	// Checkout original branch
	if _, err := gitCmd("checkout", state.OriginalBranch); err != nil {
		return fmt.Errorf("checkout original branch %s: %w", state.OriginalBranch, err)
	}

	// Merge the worktree branch
	_, err := gitCmd("merge", "--no-ff", "-m", fmt.Sprintf("iter: merge %s", state.WorktreeBranch), state.WorktreeBranch)
	if err != nil {
		return fmt.Errorf("merge worktree branch: %w", err)
	}

	return nil
}

// cleanupWorktree removes the worktree and optionally the branch.
func cleanupWorktree(state *State, deleteBranch bool) error {
	if state.WorktreePath == "" {
		return nil
	}

	// Remove the worktree
	if _, err := gitCmd("worktree", "remove", "--force", state.WorktreePath); err != nil {
		// Try manual removal if git worktree remove fails
		_ = os.RemoveAll(state.WorktreePath)
	}

	// Prune worktree references
	_, _ = gitCmd("worktree", "prune")

	// Optionally delete the branch
	if deleteBranch && state.WorktreeBranch != "" {
		_, _ = gitCmd("branch", "-D", state.WorktreeBranch)
	}

	return nil
}

const (
	stateDir      = ".iter"
	stateFile     = "state.json"
	daemonPIDFile = "index.pid"
	daemonLogFile = "index.log"
)

// version is set via -ldflags at build time
var version = "dev"

// findProjectRoot searches upward from cwd for project markers (.git, go.mod).
// Returns the directory containing the marker, or cwd if none found.
func findProjectRoot() string {
	cwd, err := os.Getwd()
	if err != nil {
		return "."
	}

	dir := cwd
	for {
		// Check for project markers
		for _, marker := range []string{".git", "go.mod", ".claude-plugin"} {
			if _, err := os.Stat(filepath.Join(dir, marker)); err == nil {
				return dir
			}
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached filesystem root, use original cwd
			return cwd
		}
		dir = parent
	}
}

// getStateDir returns the path to the .iter directory in the project root.
func getStateDir() string {
	return filepath.Join(findProjectRoot(), stateDir)
}

// Embedded prompts - all prompt content lives here in the binary
var prompts = struct {
	SystemRules     string
	ArchitectRole   string
	WorkerRole      string
	ValidatorRole   string
	WorkflowSystem  string
	ValidationRules string
}{
	SystemRules: `## EXECUTION RULES

These rules are NON-NEGOTIABLE:

1. **CORRECTNESS OVER SPEED** - Never rush. Quality is mandatory.
2. **REQUIREMENTS ARE LAW** - No interpretation, no deviation, no "improvements"
3. **EXISTING PATTERNS ARE LAW** - Match codebase style exactly
4. **BUILD MUST PASS** - Verify after every change
5. **CLEANUP IS MANDATORY** - Remove dead code, no orphaned artifacts
6. **TESTS MUST PASS** - All existing tests, plus new tests for new code`,

	ArchitectRole: `## ARCHITECT PHASE

You are analyzing requirements and creating an implementation plan.

### Instructions
1. **Analyze Codebase First**: Examine existing patterns, conventions, architecture
2. **Extract Requirements**: List ALL explicit and implicit requirements
3. **Identify Cleanup**: Find dead code, redundant patterns, technical debt
4. **Create Step Plan**: Break work into discrete, verifiable steps

### Output Files (create in .iter/workdir/)

**requirements.md**
- Unique IDs: R1, R2, R3...
- Priority: MUST | SHOULD | MAY
- Include implicit requirements (error handling, tests, docs)

**step_N.md** (one per step)
- Title: Brief description
- Requirements: Which R-IDs this addresses
- Approach: Detailed implementation plan
- Cleanup: Dead code to remove
- Acceptance: How to verify completion

**architect-analysis.md**
- Patterns found in codebase
- Architectural decisions made
- Risk assessment`,

	WorkerRole: `## WORKER PHASE

You are implementing the current step exactly as specified.

### CRITICAL RULES
- Follow step document EXACTLY - no interpretation
- Make ONLY the changes specified
- Verify build passes after each change
- Perform ALL cleanup listed in step doc
- Document what you did in step_N_impl.md

### Workflow
1. Read .iter/workdir/step_N.md
2. Implement exactly as specified
3. Run build verification
4. Create step_N_impl.md with summary
5. Request validation`,

	ValidatorRole: `## VALIDATOR PHASE

**DEFAULT STANCE: REJECT**

Your job is to find problems. Assume implementation is wrong until proven correct.

### Validation Checklist

**1. BUILD VERIFICATION** (AUTO-REJECT if fails)
- Run build command
- Run tests
- Run linter

**2. REQUIREMENTS TRACEABILITY** (AUTO-REJECT if missing)
- Every change must trace to a requirement
- Every requirement for this step must be implemented

**3. CODE QUALITY**
- Changes match existing patterns
- Error handling is complete
- No debug code left behind
- Proper logging (not print statements)

**4. CLEANUP VERIFICATION**
- All cleanup items addressed
- No new dead code introduced
- No orphaned files

**5. STEP COMPLETION**
- All acceptance criteria met
- Changes are minimal and focused
- No scope creep

### Verdict
After review, call:
- 'iter pass' - ALL checks pass
- 'iter reject "specific reason"' - ANY check fails`,

	WorkflowSystem: `## WORKFLOW MODE

You are executing a custom workflow specification.

The workflow defines:
- Phases/stages to execute
- Success criteria for each phase
- Iteration rules

Parse the workflow spec and execute each phase in order.
Track progress and iterate until all success criteria are met.`,

	ValidationRules: `### Auto-Reject Conditions
- Build fails
- Tests fail
- Lint errors
- Missing requirement traceability
- Dead code not cleaned up
- Acceptance criteria not met

### Pass Conditions
- ALL checklist items verified
- Build passes
- Tests pass
- Requirements traced
- Cleanup complete`,
}

// State represents the persistent state of an iter session.
type State struct {
	Task           string    `json:"task"`
	Mode           string    `json:"mode"` // "iter" or "workflow"
	WorkflowSpec   string    `json:"workflow_spec,omitempty"`
	Workdir        string    `json:"workdir"`
	MaxIterations  int       `json:"max_iterations"`
	Iteration      int       `json:"iteration"`
	Phase          string    `json:"phase"` // architect, worker, validator, complete
	CurrentStep    int       `json:"current_step"`
	TotalSteps     int       `json:"total_steps"`
	StartedAt      time.Time `json:"started_at"`
	LastActivityAt time.Time `json:"last_activity_at"`
	ValidationPass int       `json:"validation_pass"`
	Rejections     int       `json:"rejections"`
	ExitSignal     bool      `json:"exit_signal"`
	Completed      bool      `json:"completed"`
	Artifacts      []string  `json:"artifacts"`
	Verdicts       []Verdict `json:"verdicts"`
	// Git worktree fields
	OriginalBranch  string `json:"original_branch,omitempty"`
	WorktreeBranch  string `json:"worktree_branch,omitempty"`
	WorktreePath    string `json:"worktree_path,omitempty"`
	OriginalWorkdir string `json:"original_workdir,omitempty"`
	OSInfo          OSInfo `json:"os_info"`
}

// Verdict represents a validator verdict.
type Verdict struct {
	Step      int       `json:"step"`
	Pass      int       `json:"pass"`
	Status    string    `json:"status"` // pass, reject
	Reasons   []string  `json:"reasons"`
	Timestamp time.Time `json:"timestamp"`
}

// HookResponse is the JSON output format for Claude Code hooks.
type HookResponse struct {
	Continue           bool                `json:"continue"`
	SuppressOutput     bool                `json:"suppressOutput,omitempty"`
	SystemMessage      string              `json:"systemMessage,omitempty"`
	HookSpecificOutput *HookSpecificOutput `json:"hookSpecificOutput,omitempty"`
}

// HookSpecificOutput provides hook-specific control.
type HookSpecificOutput struct {
	HookEventName      string         `json:"hookEventName,omitempty"`
	PermissionDecision string         `json:"permissionDecision,omitempty"`
	AdditionalContext  string         `json:"additionalContext,omitempty"`
	UpdatedInput       map[string]any `json:"updatedInput,omitempty"`
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	// Auto-start index daemon for commands that benefit from it
	// Skip for: daemon stop, version, help, hook-stop, reset
	if shouldAutoStartDaemon(cmd, args) {
		repoRoot, err := os.Getwd()
		if err == nil {
			cfg := index.DefaultConfig(repoRoot)
			ensureIndexDaemon(cfg)
		}
	}

	var err error
	switch cmd {
	case "run":
		err = cmdRun(args)
	case "workflow":
		err = cmdWorkflow(args)
	case "status":
		err = cmdStatus(args)
	case "step":
		err = cmdStep(args)
	case "pass":
		err = cmdPass(args)
	case "reject":
		err = cmdReject(args)
	case "next":
		err = cmdNext(args)
	case "complete":
		err = cmdComplete(args)
	case "reset":
		err = cmdReset(args)
	case "hook-stop":
		err = cmdHookStop(args)
	case "index":
		err = cmdIndex(args)
	case "search":
		err = cmdSearch(args)
	case "version", "-v", "--version":
		cmdVersion()
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", cmd)
		printUsage()
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// shouldAutoStartDaemon determines if the daemon should be auto-started for the given command.
func shouldAutoStartDaemon(cmd string, args []string) bool {
	switch cmd {
	case "run", "workflow", "search", "status":
		return true
	case "index":
		// Don't auto-start for "index daemon stop" or "index daemon --foreground"
		if len(args) > 0 && args[0] == "daemon" {
			if len(args) > 1 && args[1] == "stop" {
				return false
			}
			// Don't auto-start when we're explicitly starting/checking the daemon
			return false
		}
		return true
	default:
		return false
	}
}

func printUsage() {
	fmt.Println(`iter - Adversarial iterative implementation for Claude Code

Commands:
  run "<task>"           Start iterative implementation until requirements/tests pass
    --max-iterations N   Set maximum iterations (default 50)
    --no-worktree        Disable git worktree isolation
  workflow "<spec>"      Start workflow-based implementation
    --max-iterations N   Set maximum iterations (default 50)
    --no-worktree        Disable git worktree isolation
  status                 Show current session status
  step [N]               Show current/specific step instructions
  pass                   Record validation pass
  reject "<reason>"      Record validation rejection
  next                   Move to next step
  complete               Mark session complete (merges worktree if active)
  reset                  Reset session state (cleans up worktree)
  hook-stop              Stop hook handler (JSON output)
  version                Show version
  help                   Show this help

Code Index Commands:
  index                  Show index status
  index build            Build/rebuild the full code index
  index clear            Clear and rebuild the index
  index watch            Start file watcher for real-time indexing (blocking)
  index daemon           Start background daemon (auto-detaches)
  index daemon status    Check if daemon is running
  index daemon stop      Stop the daemon gracefully
  search "<query>"       Search the code index
    --kind=<type>        Filter by symbol type (function, method, type, const)
    --path=<prefix>      Filter by file path prefix
    --branch=<branch>    Filter by git branch
    --limit=<n>          Maximum results (default 10)

Index Daemon:
  The index daemon runs as a persistent background process that watches
  for file changes and keeps the code index updated in real-time.
  It auto-starts when running: iter run, workflow, search, status, or index.

The iter loop:
  1. ARCHITECT: Analyze requirements, create step documents
  2. WORKER: Implement each step exactly as specified
  3. VALIDATOR: Review with adversarial stance (default REJECT)
  4. Loop until all steps pass or max iterations reached

Git Worktree:
  By default, iter creates an isolated git worktree for each session.
  This ensures changes don't affect your main branch until completion.
  Use --no-worktree to disable this behavior.

OS Detection:
  iter automatically detects the operating system (Linux, macOS, Windows, WSL)
  and provides environment-specific guidance to Claude.`)
}

func cmdVersion() {
	fmt.Printf("iter version %s\n", version)
}

// summarizeTask creates a short slug from a task description.
// It extracts the first few meaningful words, converts to lowercase,
// and uses hyphens as separators.
func summarizeTask(task string) string {
	// Convert to lowercase
	s := strings.ToLower(task)

	// Replace newlines and multiple spaces with single space
	s = regexp.MustCompile(`[\r\n]+`).ReplaceAllString(s, " ")
	s = regexp.MustCompile(`\s+`).ReplaceAllString(s, " ")
	s = strings.TrimSpace(s)

	// Extract words (alphanumeric only)
	words := regexp.MustCompile(`[a-z0-9]+`).FindAllString(s, -1)

	// Take first 5 words max
	if len(words) > 5 {
		words = words[:5]
	}

	// Join with hyphens
	slug := strings.Join(words, "-")

	// Truncate to 40 chars max
	if len(slug) > 40 {
		slug = slug[:40]
		// Don't end with a hyphen
		slug = strings.TrimSuffix(slug, "-")
	}

	// Fallback if empty
	if slug == "" {
		slug = "task"
	}

	return slug
}

// generateWorkdirPath creates a unique workdir path from a task.
// If baseDir is provided, returns an absolute path under that directory.
func generateWorkdirPath(task, baseDir string) string {
	slug := summarizeTask(task)
	timestamp := time.Now().Format("20060102-03-04")
	relPath := filepath.Join(stateDir, "workdir", fmt.Sprintf("%s-%s", slug, timestamp))
	if baseDir != "" {
		return filepath.Join(baseDir, relPath)
	}
	return relPath
}

// cmdRun starts an iterative implementation session.
func cmdRun(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: iter run \"<task>\" [--max-iterations N] [--no-worktree]")
	}

	task := args[0]
	maxIterations := 50
	useWorktree := true

	for i := 1; i < len(args); i++ {
		if args[i] == "--max-iterations" && i+1 < len(args) {
			n, err := strconv.Atoi(args[i+1])
			if err != nil {
				return fmt.Errorf("invalid max-iterations: %w", err)
			}
			maxIterations = n
			i++
		}
		if args[i] == "--no-worktree" {
			useWorktree = false
		}
	}

	// Detect OS
	osInfo := detectOS()

	// Get current working directory and project root
	originalWorkdir, _ := os.Getwd()
	projectRoot := findProjectRoot()

	// Generate unique workdir path (always in project root .iter directory)
	workdir := generateWorkdirPath(task, projectRoot)
	taskSlug := summarizeTask(task)

	// Initialize state
	state := &State{
		Task:            task,
		Mode:            "iter",
		Workdir:         workdir,
		MaxIterations:   maxIterations,
		Iteration:       1,
		Phase:           "architect",
		CurrentStep:     1,
		TotalSteps:      0,
		StartedAt:       time.Now(),
		LastActivityAt:  time.Now(),
		Artifacts:       []string{},
		Verdicts:        []Verdict{},
		OriginalWorkdir: originalWorkdir,
		OSInfo:          osInfo,
	}

	// Create git worktree if in a git repo
	worktreeInfo := ""
	if useWorktree && isGitRepo() {
		// Check for uncommitted changes
		if hasUncommittedChanges() {
			fmt.Println("Warning: You have uncommitted changes. Consider committing or stashing them first.")
			fmt.Println("Proceeding with worktree creation...")
		}

		// Get current branch before creating worktree
		currentBranch, err := getCurrentBranch()
		if err != nil {
			return fmt.Errorf("failed to get current branch: %w", err)
		}
		state.OriginalBranch = currentBranch

		// Create worktree
		worktreePath, branchName, err := createWorktree(taskSlug)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to create worktree: %v\n", err)
			fmt.Fprintln(os.Stderr, "Continuing without worktree isolation...")
		} else {
			state.WorktreePath = worktreePath
			state.WorktreeBranch = branchName

			// Change to worktree directory
			if err := os.Chdir(worktreePath); err != nil {
				return fmt.Errorf("failed to change to worktree: %w", err)
			}

			worktreeInfo = fmt.Sprintf(`
## Git Worktree (ISOLATED WORKSPACE)
- **Original branch**: %s
- **Working branch**: %s
- **Worktree path**: %s

All changes will be made in this isolated worktree.
On completion, changes will be merged back to '%s'.
`, currentBranch, branchName, worktreePath, currentBranch)
		}
	}

	// Create workdir
	if err := os.MkdirAll(workdir, 0755); err != nil {
		return fmt.Errorf("failed to create workdir: %w", err)
	}

	if err := saveState(state); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	// Get OS guidance
	osGuidance := getOSGuidance(osInfo)

	// Output the full iterative implementation prompt
	fmt.Printf(`# ITERATIVE IMPLEMENTATION

## Task
%s
%s
%s

%s

%s

---

## PHASE: ARCHITECT

Begin by analyzing the codebase and creating your implementation plan.

%s

---

## Session Info
- Iteration: %d/%d
- State: .iter/state.json
- Artifacts: %s/

## Next Steps
1. Create requirements.md, step_N.md files, architect-analysis.md
2. Set total steps: Update .iter/state.json "total_steps" field
3. Then implementation begins automatically

The session will continue until all steps pass validation or max iterations reached.
Exit is blocked until completion - use 'iter complete' when done or 'iter reset' to abort.
`,
		task,
		worktreeInfo,
		osGuidance,
		prompts.SystemRules,
		prompts.ValidationRules,
		prompts.ArchitectRole,
		state.Iteration, state.MaxIterations, state.Workdir)

	return nil
}

// cmdWorkflow starts a workflow-based implementation session.
func cmdWorkflow(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: iter workflow \"<workflow-spec>\" [--max-iterations N] [--no-worktree]")
	}

	spec := args[0]
	maxIterations := 50
	useWorktree := true

	for i := 1; i < len(args); i++ {
		if args[i] == "--max-iterations" && i+1 < len(args) {
			n, err := strconv.Atoi(args[i+1])
			if err != nil {
				return fmt.Errorf("invalid max-iterations: %w", err)
			}
			maxIterations = n
			i++
		}
		if args[i] == "--no-worktree" {
			useWorktree = false
		}
	}

	// Detect OS
	osInfo := detectOS()

	// Get current working directory and project root
	originalWorkdir, _ := os.Getwd()
	projectRoot := findProjectRoot()

	// Generate unique workdir path (always in project root .iter directory)
	workdir := generateWorkdirPath(spec, projectRoot)
	taskSlug := summarizeTask(spec)

	state := &State{
		Task:            "Workflow execution",
		Mode:            "workflow",
		WorkflowSpec:    spec,
		Workdir:         workdir,
		MaxIterations:   maxIterations,
		Iteration:       1,
		Phase:           "architect",
		CurrentStep:     1,
		StartedAt:       time.Now(),
		LastActivityAt:  time.Now(),
		Artifacts:       []string{},
		Verdicts:        []Verdict{},
		OriginalWorkdir: originalWorkdir,
		OSInfo:          osInfo,
	}

	// Create git worktree if in a git repo
	worktreeInfo := ""
	if useWorktree && isGitRepo() {
		if hasUncommittedChanges() {
			fmt.Println("Warning: You have uncommitted changes. Consider committing or stashing them first.")
			fmt.Println("Proceeding with worktree creation...")
		}

		currentBranch, err := getCurrentBranch()
		if err != nil {
			return fmt.Errorf("failed to get current branch: %w", err)
		}
		state.OriginalBranch = currentBranch

		worktreePath, branchName, err := createWorktree(taskSlug)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to create worktree: %v\n", err)
			fmt.Fprintln(os.Stderr, "Continuing without worktree isolation...")
		} else {
			state.WorktreePath = worktreePath
			state.WorktreeBranch = branchName

			if err := os.Chdir(worktreePath); err != nil {
				return fmt.Errorf("failed to change to worktree: %w", err)
			}

			worktreeInfo = fmt.Sprintf(`
## Git Worktree (ISOLATED WORKSPACE)
- **Original branch**: %s
- **Working branch**: %s
- **Worktree path**: %s

All changes will be made in this isolated worktree.
On completion, changes will be merged back to '%s'.
`, currentBranch, branchName, worktreePath, currentBranch)
		}
	}

	if err := os.MkdirAll(workdir, 0755); err != nil {
		return fmt.Errorf("failed to create workdir: %w", err)
	}

	if err := saveState(state); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	// Get OS guidance
	osGuidance := getOSGuidance(osInfo)

	fmt.Printf(`# WORKFLOW EXECUTION
%s
%s

%s

%s

---

## Workflow Specification
%s

---

%s

## Instructions
1. Parse the workflow specification above
2. Identify phases/stages and success criteria
3. Execute each phase, validating success criteria
4. Iterate until all criteria are met

## Session Info
- Iteration: %d/%d
- State: .iter/state.json
- Artifacts: %s/

Use 'iter pass'/'iter reject' to record phase outcomes.
Use 'iter next' to advance phases.
Use 'iter complete' when workflow is done.
`,
		worktreeInfo,
		osGuidance,
		prompts.SystemRules,
		prompts.ValidationRules,
		spec,
		prompts.WorkflowSystem,
		state.Iteration, state.MaxIterations, state.Workdir)

	return nil
}

// cmdStatus shows current session status.
func cmdStatus(args []string) error {
	state, err := loadState()
	if err != nil {
		fmt.Println("No active iter session.")
		return nil
	}

	elapsed := time.Since(state.StartedAt).Round(time.Second)

	fmt.Printf(`# Iter Session Status

Task: %s
Mode: %s
Phase: %s
Iteration: %d/%d
Step: %d/%d
Validation Pass: %d
Rejections: %d
Elapsed: %s
Completed: %v
`, state.Task, state.Mode, state.Phase, state.Iteration, state.MaxIterations,
		state.CurrentStep, state.TotalSteps, state.ValidationPass, state.Rejections,
		elapsed, state.Completed)

	if len(state.Verdicts) > 0 {
		fmt.Println("\n## Recent Verdicts")
		start := len(state.Verdicts) - 5
		if start < 0 {
			start = 0
		}
		for _, v := range state.Verdicts[start:] {
			fmt.Printf("- Step %d Pass %d: %s\n", v.Step, v.Pass, v.Status)
			for _, r := range v.Reasons {
				fmt.Printf("  - %s\n", r)
			}
		}
	}

	return nil
}

// cmdStep outputs current or specified step instructions.
func cmdStep(args []string) error {
	state, err := loadState()
	if err != nil {
		return fmt.Errorf("no active iter session")
	}

	stepNum := state.CurrentStep
	if len(args) > 0 {
		n, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("invalid step number: %w", err)
		}
		stepNum = n
	}

	// Try to read step file
	stepFile := filepath.Join(state.Workdir, fmt.Sprintf("step_%d.md", stepNum))
	content, err := os.ReadFile(stepFile)

	fmt.Printf(`# STEP %d IMPLEMENTATION

%s

---

`, stepNum, prompts.WorkerRole)

	if err != nil {
		fmt.Printf(`## Step Document
No step_%d.md found in .iter/workdir/

If architect phase is not complete, create step documents first.

## Current Task
%s

Iteration: %d/%d
`, stepNum, state.Task, state.Iteration, state.MaxIterations)
	} else {
		fmt.Printf("## Step Document\n\n%s\n", string(content))
	}

	fmt.Printf(`
---

## After Implementation
1. Verify build passes
2. Create step_%d_impl.md documenting what you did
3. Validation will run automatically
`, stepNum)

	return nil
}

// cmdPass records a validation pass.
func cmdPass(args []string) error {
	state, err := loadState()
	if err != nil {
		return fmt.Errorf("no active iter session")
	}

	state.LastActivityAt = time.Now()
	state.Verdicts = append(state.Verdicts, Verdict{
		Step:      state.CurrentStep,
		Pass:      state.ValidationPass,
		Status:    "pass",
		Reasons:   []string{"All validation checks passed"},
		Timestamp: time.Now(),
	})

	if err := saveState(state); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	fmt.Printf("PASS: Step %d validated successfully.\n", state.CurrentStep)

	if state.CurrentStep >= state.TotalSteps && state.TotalSteps > 0 {
		fmt.Println("\nAll steps completed. Run 'iter complete' to finalize.")
	} else {
		fmt.Println("\nRun 'iter next' to proceed to next step.")
	}

	return nil
}

// cmdReject records a validation rejection.
func cmdReject(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: iter reject \"<reason>\"")
	}

	state, err := loadState()
	if err != nil {
		return fmt.Errorf("no active iter session")
	}

	reason := strings.Join(args, " ")
	state.Rejections++
	state.LastActivityAt = time.Now()
	state.Verdicts = append(state.Verdicts, Verdict{
		Step:      state.CurrentStep,
		Pass:      state.ValidationPass,
		Status:    "reject",
		Reasons:   []string{reason},
		Timestamp: time.Now(),
	})
	state.Phase = "worker" // Back to worker

	if err := saveState(state); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	fmt.Printf("REJECT: Step %d (pass %d)\nReason: %s\n", state.CurrentStep, state.ValidationPass, reason)
	fmt.Println("\nFix the issue and validation will run again.")

	return nil
}

// cmdNext moves to the next step.
func cmdNext(args []string) error {
	state, err := loadState()
	if err != nil {
		return fmt.Errorf("no active iter session")
	}

	state.CurrentStep++
	state.ValidationPass = 0
	state.Phase = "worker"
	state.LastActivityAt = time.Now()

	if err := saveState(state); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	fmt.Printf("Moved to step %d.\n", state.CurrentStep)

	if state.CurrentStep > state.TotalSteps && state.TotalSteps > 0 {
		fmt.Println("All planned steps complete. Run 'iter complete' to finalize.")
	}

	return nil
}

// cmdComplete marks the session as complete.
func cmdComplete(args []string) error {
	state, err := loadState()
	if err != nil {
		return fmt.Errorf("no active iter session")
	}

	state.Completed = true
	state.ExitSignal = true
	state.Phase = "complete"
	state.LastActivityAt = time.Now()

	// Handle git worktree merge
	if state.WorktreeBranch != "" && state.OriginalBranch != "" {
		fmt.Printf("Merging worktree branch '%s' into '%s'...\n", state.WorktreeBranch, state.OriginalBranch)

		if err := mergeWorktree(state); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to merge worktree: %v\n", err)
			fmt.Fprintln(os.Stderr, "You may need to merge manually:")
			fmt.Fprintf(os.Stderr, "  git checkout %s\n", state.OriginalBranch)
			fmt.Fprintf(os.Stderr, "  git merge %s\n", state.WorktreeBranch)
		} else {
			fmt.Println("Worktree merged successfully.")

			// Cleanup worktree (keep branch for reference)
			if err := cleanupWorktree(state, false); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to cleanup worktree: %v\n", err)
			}

			// Change back to original directory
			if state.OriginalWorkdir != "" {
				_ = os.Chdir(state.OriginalWorkdir)
			}
		}
	}

	if err := saveState(state); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	// Create summary
	summaryPath := filepath.Join(state.Workdir, "summary.md")
	summary := fmt.Sprintf(`# Iter Session Summary

## Task
%s

## Results
- Mode: %s
- Total Iterations: %d
- Steps Completed: %d/%d
- Rejections: %d
`, state.Task, state.Mode, state.Iteration, state.CurrentStep, state.TotalSteps, state.Rejections)

	if state.WorktreeBranch != "" {
		summary += fmt.Sprintf(`
## Git Info
- Original Branch: %s
- Work Branch: %s
`, state.OriginalBranch, state.WorktreeBranch)
	}

	summary += "\n## Verdicts\n"

	for _, v := range state.Verdicts {
		summary += fmt.Sprintf("\n### Step %d Pass %d: %s\n", v.Step, v.Pass, v.Status)
		for _, r := range v.Reasons {
			summary += fmt.Sprintf("- %s\n", r)
		}
	}

	if err := os.WriteFile(summaryPath, []byte(summary), 0644); err != nil {
		return fmt.Errorf("failed to write summary: %w", err)
	}

	fmt.Println("Session complete.")
	fmt.Printf("Summary: %s\n", summaryPath)

	return nil
}

// cmdReset clears session state.
func cmdReset(args []string) error {
	// Try to load state to cleanup worktree
	state, err := loadState()
	if err == nil && state.WorktreeBranch != "" {
		fmt.Println("Cleaning up worktree...")

		// Change back to original directory first
		if state.OriginalWorkdir != "" {
			_ = os.Chdir(state.OriginalWorkdir)
		}

		// Checkout original branch
		if state.OriginalBranch != "" {
			if _, err := gitCmd("checkout", state.OriginalBranch); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to checkout original branch: %v\n", err)
			}
		}

		// Cleanup worktree and delete the branch (since we're resetting)
		if err := cleanupWorktree(state, true); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to cleanup worktree: %v\n", err)
		}
	}

	if err := os.RemoveAll(stateDir); err != nil {
		return fmt.Errorf("failed to remove state: %w", err)
	}
	fmt.Println("Iter session reset.")
	return nil
}

// HookInput represents the JSON input received from Claude Code hooks via stdin.
type HookInput struct {
	SessionID         string `json:"session_id,omitempty"`
	Transcript        string `json:"transcript,omitempty"`
	TranscriptSummary string `json:"transcript_summary,omitempty"`
	StopHookComplete  bool   `json:"stop_hook_complete,omitempty"`
}

// isIterCommand checks if the transcript/prompt indicates an /iter command.
func isIterCommand(input HookInput) bool {
	// Check transcript for /iter command patterns
	transcript := input.Transcript
	if transcript == "" {
		transcript = input.TranscriptSummary
	}

	// Look for /iter commands in the transcript
	// Match /iter, /iter-workflow, but not other commands
	iterPattern := regexp.MustCompile(`(?m)^/iter(?:-workflow)?\b`)
	return iterPattern.MatchString(transcript)
}

// cmdHookStop handles the stop hook for Claude Code.
func cmdHookStop(args []string) error {
	// Read hook input from stdin
	var input HookInput
	decoder := json.NewDecoder(os.Stdin)
	_ = decoder.Decode(&input) // Ignore errors - input may be empty

	state, err := loadState()
	if err != nil {
		// No session - allow exit
		return outputJSON(HookResponse{Continue: true})
	}

	if state.Completed {
		return outputJSON(HookResponse{
			Continue:      true,
			SystemMessage: "Iter session completed.",
		})
	}

	// CRITICAL: Only block for /iter commands, not other commands like /commit
	// Check if this is actually an iter command based on transcript content
	if !isIterCommand(input) {
		// Not an iter command - allow continuation without blocking
		return outputJSON(HookResponse{Continue: true})
	}

	if state.Iteration >= state.MaxIterations {
		return outputJSON(HookResponse{
			Continue:      true,
			SystemMessage: fmt.Sprintf("Max iterations reached (%d).", state.MaxIterations),
		})
	}

	// Continue loop
	state.Iteration++
	state.LastActivityAt = time.Now()
	if err := saveState(state); err != nil {
		return outputJSON(HookResponse{
			Continue:      true,
			SystemMessage: fmt.Sprintf("Warning: state save failed: %v", err),
		})
	}

	var nextPrompt string
	switch state.Phase {
	case "architect":
		state.Phase = "worker"
		_ = saveState(state)
		nextPrompt = fmt.Sprintf(`# CONTINUE: Iteration %d/%d

Architect phase complete. Begin implementation.

%s

Read .iter/workdir/step_%d.md and implement exactly as specified.
After implementation, validation runs automatically.`,
			state.Iteration, state.MaxIterations,
			prompts.WorkerRole,
			state.CurrentStep)

	case "worker":
		state.Phase = "validator"
		state.ValidationPass++
		_ = saveState(state)
		nextPrompt = fmt.Sprintf(`# CONTINUE: Iteration %d/%d

Implementation complete. Running validation.

%s

Review step %d implementation now.
Call 'iter pass' or 'iter reject "reason"' when done.`,
			state.Iteration, state.MaxIterations,
			prompts.ValidatorRole,
			state.CurrentStep)

	case "validator":
		if len(state.Verdicts) > 0 {
			last := state.Verdicts[len(state.Verdicts)-1]
			if last.Status == "reject" {
				nextPrompt = fmt.Sprintf(`# CONTINUE: Iteration %d/%d

Step %d was REJECTED:
%s

%s

Fix the issues and validation will run again.`,
					state.Iteration, state.MaxIterations,
					state.CurrentStep,
					strings.Join(last.Reasons, "\n"),
					prompts.WorkerRole)
			} else {
				nextPrompt = fmt.Sprintf(`# CONTINUE: Iteration %d/%d

Step %d PASSED. Run 'iter next' to proceed.

Current progress: Step %d/%d`,
					state.Iteration, state.MaxIterations,
					state.CurrentStep,
					state.CurrentStep, state.TotalSteps)
			}
		} else {
			nextPrompt = fmt.Sprintf(`# CONTINUE: Iteration %d/%d

Run validation for step %d.

%s`,
				state.Iteration, state.MaxIterations,
				state.CurrentStep,
				prompts.ValidatorRole)
		}

	default:
		nextPrompt = fmt.Sprintf(`# CONTINUE: Iteration %d/%d

Phase: %s | Step: %d/%d

Run 'iter status' for session details.`,
			state.Iteration, state.MaxIterations,
			state.Phase, state.CurrentStep, state.TotalSteps)
	}

	return outputJSON(HookResponse{
		Continue:      false,
		SystemMessage: nextPrompt,
	})
}

// Code Index Commands

// ensureIndex checks if index exists and builds it if empty.
func ensureIndex(cfg index.Config) (*index.Indexer, error) {
	idx, err := index.NewIndexer(cfg)
	if err != nil {
		return nil, fmt.Errorf("create indexer: %w", err)
	}

	// Auto-build if index is empty
	if idx.Stats().DocumentCount == 0 {
		fmt.Println("Index is empty. Building automatically...")
		start := time.Now()
		if err := idx.IndexAll(); err != nil {
			return nil, fmt.Errorf("auto-build index: %w", err)
		}
		stats := idx.Stats()
		fmt.Printf("Indexed %d symbols from %d files in %s\n",
			stats.DocumentCount, stats.FileCount, time.Since(start).Round(time.Millisecond))
	}

	return idx, nil
}

// cmdIndex handles the index subcommand.
func cmdIndex(args []string) error {
	// Get current working directory as repo root
	repoRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	cfg := index.DefaultConfig(repoRoot)

	// Determine subcommand
	subCmd := ""
	if len(args) > 0 {
		subCmd = args[0]
	}

	switch subCmd {
	case "build":
		return cmdIndexBuild(cfg)
	case "clear":
		return cmdIndexClear(cfg)
	case "watch":
		return cmdIndexWatch(cfg)
	case "daemon":
		if len(args) > 1 {
			switch args[1] {
			case "status":
				return cmdDaemonStatus(cfg)
			case "stop":
				return cmdDaemonStop(cfg)
			}
		}
		// Check for --foreground flag
		foreground := false
		for _, arg := range args[1:] {
			if arg == "--foreground" {
				foreground = true
				break
			}
		}
		return cmdDaemonStart(cfg, foreground)
	default:
		return cmdIndexStatus(cfg)
	}
}

// cmdIndexStatus shows index statistics.
func cmdIndexStatus(cfg index.Config) error {
	idx, err := ensureIndex(cfg)
	if err != nil {
		return err
	}

	stats := idx.Stats()
	daemonRunning, daemonPID := isDaemonRunning(cfg)

	daemonStatus := "stopped"
	if daemonRunning {
		daemonStatus = fmt.Sprintf("running (PID %d)", daemonPID)
	}

	fmt.Printf(`# Code Index Status

Documents: %d
Files indexed: %d
Current branch: %s
Last updated: %s
Daemon: %s

Index path: %s
`, stats.DocumentCount, stats.FileCount, stats.CurrentBranch,
		formatTime(stats.LastUpdated), daemonStatus,
		filepath.Join(cfg.RepoRoot, cfg.IndexPath))

	return nil
}

// cmdIndexBuild performs a full repository index.
func cmdIndexBuild(cfg index.Config) error {
	fmt.Println("Building code index...")

	idx, err := index.NewIndexer(cfg)
	if err != nil {
		return fmt.Errorf("create indexer: %w", err)
	}

	start := time.Now()
	if err := idx.IndexAll(); err != nil {
		return fmt.Errorf("index all: %w", err)
	}

	stats := idx.Stats()
	fmt.Printf("Indexed %d symbols from %d files in %s\n",
		stats.DocumentCount, stats.FileCount, time.Since(start).Round(time.Millisecond))

	return nil
}

// cmdIndexClear clears and rebuilds the index.
func cmdIndexClear(cfg index.Config) error {
	fmt.Println("Clearing index...")

	idx, err := index.NewIndexer(cfg)
	if err != nil {
		return fmt.Errorf("create indexer: %w", err)
	}

	if err := idx.Clear(); err != nil {
		return fmt.Errorf("clear index: %w", err)
	}

	fmt.Println("Index cleared. Run 'iter index build' to rebuild.")
	return nil
}

// cmdIndexWatch starts the file watcher.
func cmdIndexWatch(cfg index.Config) error {
	fmt.Println("Starting file watcher...")

	idx, err := index.NewIndexer(cfg)
	if err != nil {
		return fmt.Errorf("create indexer: %w", err)
	}

	watcher, err := index.NewWatcher(idx)
	if err != nil {
		return fmt.Errorf("create watcher: %w", err)
	}

	if err := watcher.Start(); err != nil {
		return fmt.Errorf("start watcher: %w", err)
	}

	fmt.Printf("Watching for changes in %s\n", cfg.RepoRoot)
	fmt.Println("Press Ctrl+C to stop.")

	// Block until interrupted
	select {}
}

// Daemon PID file helpers

// getDaemonPIDPath returns the path to the daemon PID file.
func getDaemonPIDPath(cfg index.Config) string {
	return filepath.Join(cfg.RepoRoot, stateDir, daemonPIDFile)
}

// getDaemonLogPath returns the path to the daemon log file.
func getDaemonLogPath(cfg index.Config) string {
	return filepath.Join(cfg.RepoRoot, stateDir, daemonLogFile)
}

// writeDaemonPID writes the current process PID to the PID file.
func writeDaemonPID(cfg index.Config) error {
	pidPath := getDaemonPIDPath(cfg)
	if err := os.MkdirAll(filepath.Dir(pidPath), 0755); err != nil {
		return err
	}
	return os.WriteFile(pidPath, []byte(strconv.Itoa(os.Getpid())), 0644)
}

// removeDaemonPID removes the PID file.
func removeDaemonPID(cfg index.Config) {
	_ = os.Remove(getDaemonPIDPath(cfg))
}

// readDaemonPID reads the daemon PID from the PID file.
func readDaemonPID(cfg index.Config) (int, error) {
	data, err := os.ReadFile(getDaemonPIDPath(cfg))
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(data)))
}

// isDaemonRunning checks if the daemon process is running.
func isDaemonRunning(cfg index.Config) (bool, int) {
	pid, err := readDaemonPID(cfg)
	if err != nil {
		return false, 0
	}

	// Check if process exists by sending signal 0
	process, err := os.FindProcess(pid)
	if err != nil {
		return false, 0
	}

	// On Unix, sending signal 0 checks if process exists
	err = process.Signal(syscall.Signal(0))
	if err != nil {
		// Process doesn't exist, clean up stale PID file
		removeDaemonPID(cfg)
		return false, 0
	}

	return true, pid
}

// cmdDaemonStart starts the index daemon.
func cmdDaemonStart(cfg index.Config, foreground bool) error {
	// Check if already running
	if running, pid := isDaemonRunning(cfg); running {
		fmt.Printf("Index daemon already running (PID %d)\n", pid)
		return nil
	}

	if foreground {
		// Run in foreground mode with signal handling
		return runDaemonForeground(cfg)
	}

	// Spawn self as detached background process
	return spawnDaemonBackground(cfg)
}

// spawnDaemonBackground spawns the daemon as a detached background process.
func spawnDaemonBackground(cfg index.Config) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("get executable path: %w", err)
	}

	// Ensure .iter directory exists
	iterDir := filepath.Join(cfg.RepoRoot, stateDir)
	if err := os.MkdirAll(iterDir, 0755); err != nil {
		return fmt.Errorf("create .iter directory: %w", err)
	}

	// Open log file for daemon output
	logPath := getDaemonLogPath(cfg)
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("open log file: %w", err)
	}

	cmd := exec.Command(exe, "index", "daemon", "--foreground")
	cmd.Dir = cfg.RepoRoot
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	// Detach from parent process
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true, // Create new session (Linux/macOS)
	}

	if err := cmd.Start(); err != nil {
		logFile.Close()
		return fmt.Errorf("start daemon: %w", err)
	}

	// Don't wait for the process - it runs independently
	logFile.Close()

	fmt.Printf("Index daemon started (PID %d)\n", cmd.Process.Pid)
	fmt.Printf("Log file: %s\n", logPath)

	return nil
}

// runDaemonForeground runs the daemon in foreground mode with signal handling.
func runDaemonForeground(cfg index.Config) error {
	// Write timestamp to log
	fmt.Printf("[%s] Index daemon starting (PID %d)\n", time.Now().Format(time.RFC3339), os.Getpid())

	// Ensure index exists
	idx, err := ensureIndex(cfg)
	if err != nil {
		return fmt.Errorf("ensure index: %w", err)
	}

	// Write PID file
	if err := writeDaemonPID(cfg); err != nil {
		return fmt.Errorf("write PID file: %w", err)
	}
	defer removeDaemonPID(cfg)

	// Start watcher
	watcher, err := index.NewWatcher(idx)
	if err != nil {
		return fmt.Errorf("create watcher: %w", err)
	}

	if err := watcher.Start(); err != nil {
		return fmt.Errorf("start watcher: %w", err)
	}
	defer watcher.Stop()

	fmt.Printf("[%s] Watching for changes in %s\n", time.Now().Format(time.RFC3339), cfg.RepoRoot)

	// Wait for termination signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	sig := <-sigCh
	fmt.Printf("[%s] Received signal %v, shutting down...\n", time.Now().Format(time.RFC3339), sig)

	return nil
}

// cmdDaemonStatus checks if the daemon is running.
func cmdDaemonStatus(cfg index.Config) error {
	running, pid := isDaemonRunning(cfg)

	if running {
		fmt.Printf("Index daemon: running (PID %d)\n", pid)

		// Show additional info
		logPath := getDaemonLogPath(cfg)
		if info, err := os.Stat(logPath); err == nil {
			fmt.Printf("Log file: %s (%.1f KB)\n", logPath, float64(info.Size())/1024)
		}
	} else {
		fmt.Println("Index daemon: stopped")
	}

	return nil
}

// cmdDaemonStop stops the running daemon.
func cmdDaemonStop(cfg index.Config) error {
	running, pid := isDaemonRunning(cfg)
	if !running {
		fmt.Println("Index daemon is not running")
		return nil
	}

	// Find the process
	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("find process: %w", err)
	}

	// Send SIGTERM
	fmt.Printf("Stopping index daemon (PID %d)...\n", pid)
	if err := process.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("send signal: %w", err)
	}

	// Wait briefly for the process to exit
	for i := 0; i < 10; i++ {
		time.Sleep(100 * time.Millisecond)
		if running, _ := isDaemonRunning(cfg); !running {
			fmt.Println("Index daemon stopped")
			return nil
		}
	}

	// Force kill if still running
	fmt.Println("Daemon did not stop gracefully, forcing...")
	if err := process.Kill(); err != nil {
		return fmt.Errorf("kill process: %w", err)
	}

	removeDaemonPID(cfg)
	fmt.Println("Index daemon stopped")

	return nil
}

// ensureIndexDaemon ensures the index daemon is running.
// Called automatically by commands that benefit from the index.
func ensureIndexDaemon(cfg index.Config) {
	if running, _ := isDaemonRunning(cfg); running {
		return // Already running
	}

	// Spawn daemon silently in background
	if err := spawnDaemonBackground(cfg); err != nil {
		// Non-fatal: just log to stderr
		fmt.Fprintf(os.Stderr, "warning: failed to start index daemon: %v\n", err)
	}
}

// cmdSearch searches the code index.
func cmdSearch(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: iter search \"<query>\" [--kind=<type>] [--path=<prefix>] [--branch=<branch>] [--limit=<n>]")
	}

	query := args[0]
	opts := index.SearchOptions{
		Query: query,
		Limit: 10,
	}

	// Parse optional flags
	for i := 1; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "--kind=") {
			opts.SymbolKind = strings.TrimPrefix(arg, "--kind=")
		} else if strings.HasPrefix(arg, "--path=") {
			opts.FilePath = strings.TrimPrefix(arg, "--path=")
		} else if strings.HasPrefix(arg, "--branch=") {
			opts.Branch = strings.TrimPrefix(arg, "--branch=")
		} else if strings.HasPrefix(arg, "--limit=") {
			n, err := strconv.Atoi(strings.TrimPrefix(arg, "--limit="))
			if err == nil && n > 0 {
				opts.Limit = n
			}
		}
	}

	// Get current working directory as repo root
	repoRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	cfg := index.DefaultConfig(repoRoot)

	// Auto-build index if needed
	idx, err := ensureIndex(cfg)
	if err != nil {
		return err
	}

	searcher := index.NewSearcher(idx)
	results, err := searcher.Search(context.Background(), opts)
	if err != nil {
		return fmt.Errorf("search: %w", err)
	}

	// Output formatted results
	fmt.Println(index.FormatResults(results))

	return nil
}

// formatTime formats a time for display.
func formatTime(t time.Time) string {
	if t.IsZero() {
		return "never"
	}
	return t.Format(time.RFC3339)
}

// State persistence

func loadState() (*State, error) {
	// Try current directory first
	statePath := filepath.Join(stateDir, stateFile)
	data, err := os.ReadFile(statePath)
	if err != nil {
		return nil, err
	}
	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}
	return &state, nil
}

func saveState(state *State) error {
	// Always save to OriginalWorkdir if set, otherwise current directory
	baseDir := ""
	if state.OriginalWorkdir != "" {
		baseDir = state.OriginalWorkdir
	}
	return saveStateTo(state, baseDir)
}

// saveStateTo saves state to a specific base directory.
func saveStateTo(state *State, baseDir string) error {
	var statePath string
	if baseDir != "" {
		statePath = filepath.Join(baseDir, stateDir)
	} else {
		statePath = stateDir
	}

	if err := os.MkdirAll(statePath, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(statePath, stateFile), data, 0644)
}

func outputJSON(v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}
