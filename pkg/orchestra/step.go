package orchestra

// Step represents a single implementation step.
type Step struct {
	// Number is the step sequence.
	Number int

	// Title is the step name.
	Title string

	// Dependencies are other step numbers that must complete first.
	Dependencies []int

	// Requirements are REQ IDs addressed by this step.
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

// Change represents a file modification.
type Change struct {
	// Type is the modification type.
	Type ChangeType

	// Path is the file path.
	Path string

	// Before is the previous content.
	Before string

	// After is the new content.
	After string

	// Diff is the unified diff.
	Diff string

	// LineNum is the starting line for partial changes.
	LineNum int
}

// ChangeType indicates the type of file modification.
type ChangeType string

const (
	ChangeCreate ChangeType = "create"
	ChangeModify ChangeType = "modify"
	ChangeDelete ChangeType = "delete"
	ChangeRename ChangeType = "rename"
)

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

	// Document is the step_N_impl.md content.
	Document string
}

// NewStep creates a new step.
func NewStep(number int, title string) *Step {
	return &Step{
		Number: number,
		Title:  title,
	}
}

// WithDependencies sets step dependencies.
func (s *Step) WithDependencies(deps ...int) *Step {
	s.Dependencies = deps
	return s
}

// WithRequirements sets requirements addressed.
func (s *Step) WithRequirements(reqs ...string) *Step {
	s.Requirements = reqs
	return s
}

// WithApproach sets implementation approach.
func (s *Step) WithApproach(approach string) *Step {
	s.Approach = approach
	return s
}

// AddCleanup adds a cleanup item.
func (s *Step) AddCleanup(item CleanupItem) *Step {
	s.Cleanup = append(s.Cleanup, item)
	return s
}

// AddAcceptanceCriterion adds an acceptance criterion.
func (s *Step) AddAcceptanceCriterion(criterion string) *Step {
	s.AcceptanceCriteria = append(s.AcceptanceCriteria, criterion)
	return s
}

// AddVerificationCommand adds a verification command.
func (s *Step) AddVerificationCommand(cmd string) *Step {
	s.VerificationCommands = append(s.VerificationCommands, cmd)
	return s
}

// AddFile adds a file to modify.
func (s *Step) AddFile(path string) *Step {
	s.Files = append(s.Files, path)
	return s
}

// HasDependency checks if this step depends on another.
func (s *Step) HasDependency(stepNum int) bool {
	for _, dep := range s.Dependencies {
		if dep == stepNum {
			return true
		}
	}
	return false
}

// IsIndependent returns true if this step has no dependencies.
func (s *Step) IsIndependent() bool {
	return len(s.Dependencies) == 0
}

// ToDocument generates the step_N.md content.
func (s *Step) ToDocument() string {
	var sb stringBuilder

	sb.WriteString("# Step " + itoa(s.Number) + ": " + s.Title + "\n\n")

	// Dependencies
	sb.WriteString("## Dependencies\n")
	if len(s.Dependencies) == 0 {
		sb.WriteString("none\n\n")
	} else {
		for _, dep := range s.Dependencies {
			sb.WriteString("- step_" + itoa(dep) + "\n")
		}
		sb.WriteString("\n")
	}

	// Requirements
	sb.WriteString("## Requirements Addressed\n")
	for _, req := range s.Requirements {
		sb.WriteString("- " + req + "\n")
	}
	sb.WriteString("\n")

	// Approach
	sb.WriteString("## Approach\n")
	sb.WriteString(s.Approach + "\n\n")

	// Files
	if len(s.Files) > 0 {
		sb.WriteString("### Files to Modify\n")
		for _, f := range s.Files {
			sb.WriteString("- " + f + "\n")
		}
		sb.WriteString("\n")
	}

	// Cleanup
	if len(s.Cleanup) > 0 {
		sb.WriteString("## Cleanup Required\n")
		for _, item := range s.Cleanup {
			sb.WriteString("- Remove: " + item.Name + " (" + item.Type + ")\n")
			sb.WriteString("  - File: " + item.File + "\n")
			sb.WriteString("  - Reason: " + item.Reason + "\n")
		}
		sb.WriteString("\n")
	}

	// Acceptance Criteria
	sb.WriteString("## Acceptance Criteria\n")
	for i, ac := range s.AcceptanceCriteria {
		sb.WriteString("- AC-" + itoa(i+1) + ": " + ac + "\n")
	}
	sb.WriteString("\n")

	// Verification Commands
	sb.WriteString("## Verification Commands\n")
	sb.WriteString("```bash\n")
	for _, cmd := range s.VerificationCommands {
		sb.WriteString(cmd + "\n")
	}
	sb.WriteString("```\n")

	return sb.String()
}

// ParseStep parses a step document into a Step.
func ParseStep(content string) (*Step, error) {
	step := &Step{}
	step.Document = content

	// Parse title
	lines := splitLines(content)
	for _, line := range lines {
		if hasPrefix(line, "# Step ") {
			// Extract number and title
			rest := trimPrefix(line, "# Step ")
			colonIdx := indexOf(rest, ":")
			if colonIdx > 0 {
				step.Number = atoi(trimSpace(rest[:colonIdx]))
				step.Title = trimSpace(rest[colonIdx+1:])
			}
			break
		}
	}

	// Parse sections
	sections := parseSections(content)

	// Dependencies
	if deps, ok := sections["dependencies"]; ok {
		for _, line := range splitLines(deps) {
			line = trimSpace(line)
			if hasPrefix(line, "- step_") {
				num := atoi(trimPrefix(line, "- step_"))
				if num > 0 {
					step.Dependencies = append(step.Dependencies, num)
				}
			}
		}
	}

	// Requirements
	if reqs, ok := sections["requirements addressed"]; ok {
		for _, line := range splitLines(reqs) {
			line = trimSpace(line)
			if hasPrefix(line, "- ") {
				step.Requirements = append(step.Requirements, trimPrefix(line, "- "))
			}
		}
	}

	// Approach
	if approach, ok := sections["approach"]; ok {
		step.Approach = trimSpace(approach)
	}

	// Acceptance Criteria
	if ac, ok := sections["acceptance criteria"]; ok {
		for _, line := range splitLines(ac) {
			line = trimSpace(line)
			if hasPrefix(line, "- AC-") {
				colonIdx := indexOf(line, ":")
				if colonIdx > 0 {
					step.AcceptanceCriteria = append(step.AcceptanceCriteria, trimSpace(line[colonIdx+1:]))
				}
			}
		}
	}

	return step, nil
}

// Helper functions

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

func trimPrefix(s, prefix string) string {
	if hasPrefix(s, prefix) {
		return s[len(prefix):]
	}
	return s
}

func trimSpace(s string) string {
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\r' || s[start] == '\n') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\r' || s[end-1] == '\n') {
		end--
	}
	return s[start:end]
}

func indexOf(s string, sub string) int {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func atoi(s string) int {
	s = trimSpace(s)
	n := 0
	for i := 0; i < len(s); i++ {
		if s[i] >= '0' && s[i] <= '9' {
			n = n*10 + int(s[i]-'0')
		} else {
			break
		}
	}
	return n
}

func parseSections(content string) map[string]string {
	sections := make(map[string]string)
	lines := splitLines(content)

	var currentSection string
	var sectionContent stringBuilder

	for _, line := range lines {
		if hasPrefix(line, "## ") {
			if currentSection != "" {
				sections[currentSection] = sectionContent.String()
			}
			currentSection = toLower(trimPrefix(line, "## "))
			sectionContent = stringBuilder{}
		} else if currentSection != "" {
			sectionContent.WriteString(line + "\n")
		}
	}

	if currentSection != "" {
		sections[currentSection] = sectionContent.String()
	}

	return sections
}

func toLower(s string) string {
	b := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		b[i] = c
	}
	return string(b)
}
