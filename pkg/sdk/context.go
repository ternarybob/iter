package sdk

import (
	"io/fs"
	"log/slog"
)

// Index provides searchable access to codebase.
// This interface is implemented by pkg/index.
type Index interface {
	// Search performs full-text search.
	Search(query string, opts SearchOptions) ([]SearchResult, error)

	// FindSymbol finds symbols by name and kind.
	FindSymbol(name string, kind SymbolKind) ([]Symbol, error)

	// GetContext retrieves relevant code chunks up to token budget.
	GetContext(query string, maxTokens int) ([]Chunk, error)

	// GetFile retrieves a file by path.
	GetFile(path string) (*File, error)
}

// SearchOptions configures search behavior.
type SearchOptions struct {
	// MaxResults limits the number of results.
	MaxResults int

	// FilePatterns filters by file glob patterns.
	FilePatterns []string

	// IncludeContent includes file content in results.
	IncludeContent bool

	// CaseSensitive enables case-sensitive matching.
	CaseSensitive bool
}

// SearchResult represents a search match.
type SearchResult struct {
	// Path is the file path.
	Path string

	// Line is the line number (1-indexed).
	Line int

	// Content is the matching line content.
	Content string

	// Score is the relevance score.
	Score float64

	// Context contains surrounding lines.
	Context []string
}

// SymbolKind categorizes code symbols.
type SymbolKind string

const (
	SymbolKindFunction  SymbolKind = "function"
	SymbolKindMethod    SymbolKind = "method"
	SymbolKindClass     SymbolKind = "class"
	SymbolKindInterface SymbolKind = "interface"
	SymbolKindStruct    SymbolKind = "struct"
	SymbolKindVariable  SymbolKind = "variable"
	SymbolKindConstant  SymbolKind = "constant"
	SymbolKindType      SymbolKind = "type"
	SymbolKindPackage   SymbolKind = "package"
	SymbolKindModule    SymbolKind = "module"
)

// Symbol represents a code symbol.
type Symbol struct {
	// Name is the symbol name.
	Name string

	// Kind is the symbol type.
	Kind SymbolKind

	// Path is the file path.
	Path string

	// Line is the line number.
	Line int

	// Signature is the full signature (for functions/methods).
	Signature string

	// Documentation is the symbol's doc comment.
	Documentation string
}

// Chunk represents a code fragment.
type Chunk struct {
	// ID is a unique identifier.
	ID string

	// Path is the file path.
	Path string

	// StartLine is the starting line number.
	StartLine int

	// EndLine is the ending line number.
	EndLine int

	// Content is the code content.
	Content string

	// Language is the programming language.
	Language string

	// Symbols are symbols defined in this chunk.
	Symbols []Symbol
}

// File represents an indexed file.
type File struct {
	// Path is the file path.
	Path string

	// Content is the file content.
	Content string

	// Language is the programming language.
	Language string

	// Size is the file size in bytes.
	Size int64

	// ModTime is the modification time.
	ModTime int64

	// Chunks are the code chunks in this file.
	Chunks []Chunk

	// Symbols are symbols defined in this file.
	Symbols []Symbol
}

// LLMRouter provides access to language models.
// This interface is implemented by pkg/llm.
type LLMRouter interface {
	// Complete generates a completion.
	Complete(req CompletionRequest) (*CompletionResponse, error)

	// Stream generates a streaming completion.
	Stream(req CompletionRequest) (<-chan StreamChunk, error)

	// CountTokens counts tokens in content.
	CountTokens(content string) (int, error)

	// ForPlanning returns a provider configured for planning.
	ForPlanning() LLMProvider

	// ForExecution returns a provider configured for execution.
	ForExecution() LLMProvider

	// ForValidation returns a provider configured for validation.
	ForValidation() LLMProvider
}

// LLMProvider is a single LLM provider.
type LLMProvider interface {
	// Name returns the provider name.
	Name() string

	// Complete generates a completion.
	Complete(req CompletionRequest) (*CompletionResponse, error)

	// Stream generates a streaming completion.
	Stream(req CompletionRequest) (<-chan StreamChunk, error)

	// CountTokens counts tokens in content.
	CountTokens(content string) (int, error)
}

// CompletionRequest is a request to an LLM.
type CompletionRequest struct {
	// Model is the model identifier.
	Model string

	// Messages is the conversation.
	Messages []Message

	// System is the system prompt.
	System string

	// MaxTokens limits the response.
	MaxTokens int

	// Temperature controls randomness.
	Temperature float64

	// TopP is nucleus sampling parameter.
	TopP float64

	// StopWords are stop sequences.
	StopWords []string

	// Tools are available function definitions.
	Tools []Tool

	// ToolChoice controls tool selection.
	ToolChoice string
}

// Message is a conversation message.
type Message struct {
	// Role is the message role (user, assistant, system).
	Role string

	// Content is the message content.
	Content string

	// ToolCalls are tool invocations (for assistant messages).
	ToolCalls []ToolCall

	// ToolCallID links to a tool call (for tool result messages).
	ToolCallID string
}

// Tool defines a function the LLM can call.
type Tool struct {
	// Name is the function name.
	Name string

	// Description describes what the function does.
	Description string

	// Parameters is the JSON schema for parameters.
	Parameters map[string]any
}

// ToolCall represents a function invocation.
type ToolCall struct {
	// ID is the call identifier.
	ID string

	// Name is the function name.
	Name string

	// Arguments are the function arguments (JSON).
	Arguments string
}

// CompletionResponse is the LLM response.
type CompletionResponse struct {
	// Content is the text response.
	Content string

	// ToolCalls are function invocations.
	ToolCalls []ToolCall

	// FinishReason indicates why generation stopped.
	FinishReason string

	// Usage contains token counts.
	Usage TokenUsage
}

// TokenUsage tracks token consumption.
type TokenUsage struct {
	// PromptTokens is input token count.
	PromptTokens int

	// CompletionTokens is output token count.
	CompletionTokens int

	// TotalTokens is the sum.
	TotalTokens int
}

// StreamChunk is a streaming response fragment.
type StreamChunk struct {
	// Content is the text fragment.
	Content string

	// ToolCall is a partial tool call.
	ToolCall *ToolCall

	// Done indicates stream completion.
	Done bool

	// Error is any streaming error.
	Error error
}

// Session provides conversation history and state.
// This interface is implemented by pkg/session.
type Session interface {
	// ID returns the session identifier.
	ID() string

	// History returns conversation messages.
	History() []Message

	// AddMessage appends a message.
	AddMessage(msg Message)

	// GetState retrieves stored state.
	GetState(key string) (any, bool)

	// SetState stores state.
	SetState(key string, value any)

	// Clear removes all history.
	Clear()
}

// Orchestrator coordinates multi-agent workflows.
// This interface is implemented by pkg/orchestra.
type Orchestrator interface {
	// Analyze extracts requirements from a task.
	Analyze(task *Task) (*Requirements, error)

	// Plan creates step documents from requirements.
	Plan(reqs *Requirements) ([]Step, error)

	// Execute implements a single step.
	Execute(step *Step) (*StepResult, error)

	// Validate reviews an implementation (adversarial).
	Validate(step *Step, result *StepResult) (*Verdict, error)

	// FinalValidate reviews all changes together.
	FinalValidate(results []*StepResult) (*Verdict, error)

	// Iterate fixes issues from rejection.
	Iterate(step *Step, verdict *Verdict) (*StepResult, error)
}

// Requirements represents extracted requirements.
type Requirements struct {
	// Items are the individual requirements.
	Items []Requirement

	// SourceTask is the original task.
	SourceTask *Task

	// Analysis is the architect's analysis.
	Analysis string
}

// Step represents a single implementation step.
type Step struct {
	// Number is the step sequence.
	Number int

	// Title is the step name.
	Title string

	// Dependencies are other step numbers.
	Dependencies []int

	// Requirements are REQ IDs addressed.
	Requirements []string

	// Approach is implementation guidance.
	Approach string

	// Cleanup lists items to remove.
	Cleanup []CleanupItem

	// AcceptanceCriteria are verifiable conditions.
	AcceptanceCriteria []string

	// VerificationCommands are build/test commands.
	VerificationCommands []string

	// Files lists files to modify.
	Files []string

	// Document is the raw step document content.
	Document string
}

// CleanupItem represents something to remove.
type CleanupItem struct {
	// Type is the item type (function, file, type).
	Type string

	// Name is the item name.
	Name string

	// File is where it's located.
	File string

	// Reason explains why it's dead code.
	Reason string
}

// StepResult captures step execution output.
type StepResult struct {
	// StepNumber links to the step.
	StepNumber int

	// Changes are file modifications made.
	Changes []Change

	// BuildPassed indicates build success.
	BuildPassed bool

	// BuildLog is the path to build output.
	BuildLog string

	// TestsPassed indicates test success.
	TestsPassed bool

	// TestLog is the path to test output.
	TestLog string

	// Notes are implementation notes.
	Notes string

	// Iteration is which attempt this is.
	Iteration int
}

// VerdictStatus indicates validation outcome.
type VerdictStatus string

const (
	VerdictStatusPass   VerdictStatus = "pass"
	VerdictStatusReject VerdictStatus = "reject"
)

// Verdict is a validation result.
type Verdict struct {
	// Status is pass or reject.
	Status VerdictStatus

	// Reasons are rejection reasons.
	Reasons []string

	// RequirementStatus maps REQ to verification.
	RequirementStatus map[string]bool

	// AcceptanceStatus maps AC to verification.
	AcceptanceStatus map[string]bool

	// BuildPassed indicates build success.
	BuildPassed bool

	// TestsPassed indicates test success.
	TestsPassed bool

	// CleanupVerified indicates cleanup completion.
	CleanupVerified bool

	// Document is the raw validation document.
	Document string
}

// WorkdirManager handles artifact creation.
// This interface is implemented by pkg/orchestra.
type WorkdirManager interface {
	// Create creates a new workdir.
	Create(taskName string) (string, error)

	// WriteRequirements writes requirements.md.
	WriteRequirements(content string) error

	// WriteArchitectAnalysis writes architect-analysis.md.
	WriteArchitectAnalysis(content string) error

	// WriteStep writes step_N.md.
	WriteStep(n int, content string) error

	// WriteStepImpl writes step_N_impl.md.
	WriteStepImpl(n int, content string) error

	// WriteStepValidation writes step_N_valid.md.
	WriteStepValidation(n int, content string) error

	// WriteFinalValidation writes final_validation.md.
	WriteFinalValidation(content string) error

	// WriteSummary writes summary.md.
	WriteSummary(content string) error

	// WriteLog writes to logs/ subdirectory.
	WriteLog(name string, content []byte) error

	// Path returns the current workdir path.
	Path() string
}

// Config holds project configuration.
type Config struct {
	// Project contains project settings.
	Project ProjectConfig

	// Models contains model settings.
	Models ModelConfig

	// Loop contains loop settings.
	Loop LoopConfig

	// Exit contains exit detection settings.
	Exit ExitConfig

	// Circuit contains circuit breaker settings.
	Circuit CircuitConfig

	// Monitor contains monitoring settings.
	Monitor MonitorConfig
}

// ProjectConfig holds project-level settings.
type ProjectConfig struct {
	// Name is the project name.
	Name string

	// RootDir is the project root directory.
	RootDir string

	// IgnorePatterns are patterns to ignore during indexing.
	IgnorePatterns []string

	// IndexPatterns are patterns to include in indexing.
	IndexPatterns []string
}

// ModelConfig holds model settings.
type ModelConfig struct {
	// Planning is the model for architect agent.
	Planning string

	// Execution is the model for worker agent.
	Execution string

	// Validation is the model for validator agent.
	Validation string
}

// LoopConfig holds autonomous loop settings.
type LoopConfig struct {
	// MaxIterations is the forced stop limit.
	MaxIterations int

	// RateLimitPerHour is the API call limit.
	RateLimitPerHour int

	// IterationTimeout is per-iteration timeout.
	IterationTimeout string

	// Cooldown is delay between iterations.
	Cooldown string

	// MaxValidationRetries is retries before moving on.
	MaxValidationRetries int

	// ParallelSteps enables parallel independent steps.
	ParallelSteps bool
}

// ExitConfig holds exit detection settings.
type ExitConfig struct {
	// RequireExplicitSignal requires skill to set ExitSignal.
	RequireExplicitSignal bool

	// CompletionThreshold is minimum indicators for exit.
	CompletionThreshold int

	// MaxConsecutiveNoProgress is stagnation detection.
	MaxConsecutiveNoProgress int

	// MaxConsecutiveErrors is error threshold.
	MaxConsecutiveErrors int
}

// CircuitConfig holds circuit breaker settings.
type CircuitConfig struct {
	// NoProgressThreshold is loops with no file changes.
	NoProgressThreshold int

	// SameErrorThreshold is repeated error loops.
	SameErrorThreshold int

	// OutputDeclineThreshold is output decline percentage.
	OutputDeclineThreshold int

	// RecoveryTimeout is time before half-open state.
	RecoveryTimeout string
}

// MonitorConfig holds monitoring settings.
type MonitorConfig struct {
	// Enabled indicates if monitoring is active.
	Enabled bool

	// Port is the HTTP server port.
	Port int
}

// ExecutionContext provides access to all services during skill execution.
type ExecutionContext struct {
	// Codebase provides searchable access to project source code.
	Codebase Index

	// LLM provides access to language models.
	LLM LLMRouter

	// Session provides conversation history and state.
	Session Session

	// Config holds project and skill configuration.
	Config *Config

	// FS provides file system access (abstracted for testing).
	FS fs.FS

	// WorkDir is the working directory path.
	WorkDir string

	// Logger provides structured logging.
	Logger *slog.Logger

	// Iteration is the current loop iteration number.
	Iteration int

	// DryRun indicates no actual changes should be made.
	DryRun bool

	// Orchestra provides multi-agent coordination.
	Orchestra Orchestrator

	// Workdir manages artifact creation.
	Workdir WorkdirManager
}

// NewExecutionContext creates a new execution context.
func NewExecutionContext() *ExecutionContext {
	return &ExecutionContext{
		Logger: slog.Default(),
	}
}
