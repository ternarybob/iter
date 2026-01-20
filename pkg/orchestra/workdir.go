package orchestra

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// WorkdirManager handles artifact creation and retrieval.
type WorkdirManager struct {
	baseDir string
	current string
}

// NewWorkdirManager creates a new workdir manager.
func NewWorkdirManager(baseDir string) (*WorkdirManager, error) {
	if baseDir == "" {
		baseDir = "."
	}

	// Ensure .claude/workdir exists
	workdirBase := filepath.Join(baseDir, ".claude", "workdir")
	if err := os.MkdirAll(workdirBase, 0755); err != nil {
		return nil, fmt.Errorf("create workdir base: %w", err)
	}

	return &WorkdirManager{
		baseDir: workdirBase,
	}, nil
}

// Create creates a new workdir for a task.
func (m *WorkdirManager) Create(taskName string) (string, error) {
	// Format: YYYY-MM-DD-HHMM-taskName
	timestamp := time.Now().Format("2006-01-02-1504")
	dirname := timestamp + "-" + taskName

	m.current = filepath.Join(m.baseDir, dirname)
	if err := os.MkdirAll(m.current, 0755); err != nil {
		return "", fmt.Errorf("create workdir: %w", err)
	}

	// Create logs subdirectory
	logsDir := filepath.Join(m.current, "logs")
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		return "", fmt.Errorf("create logs dir: %w", err)
	}

	return m.current, nil
}

// Path returns the current workdir path.
func (m *WorkdirManager) Path() string {
	return m.current
}

// WriteRequirements writes requirements.md.
func (m *WorkdirManager) WriteRequirements(content string) error {
	return m.writeFile("requirements.md", content)
}

// WriteArchitectAnalysis writes architect-analysis.md.
func (m *WorkdirManager) WriteArchitectAnalysis(content string) error {
	return m.writeFile("architect-analysis.md", content)
}

// WriteStep writes step_N.md.
func (m *WorkdirManager) WriteStep(n int, content string) error {
	filename := fmt.Sprintf("step_%d.md", n)
	return m.writeFile(filename, content)
}

// WriteStepImpl writes step_N_impl.md.
func (m *WorkdirManager) WriteStepImpl(n int, content string) error {
	filename := fmt.Sprintf("step_%d_impl.md", n)
	return m.writeFile(filename, content)
}

// WriteStepValidation writes step_N_valid.md.
func (m *WorkdirManager) WriteStepValidation(n int, content string) error {
	filename := fmt.Sprintf("step_%d_valid.md", n)
	return m.writeFile(filename, content)
}

// WriteFinalValidation writes final_validation.md.
func (m *WorkdirManager) WriteFinalValidation(content string) error {
	return m.writeFile("final_validation.md", content)
}

// WriteSummary writes summary.md.
func (m *WorkdirManager) WriteSummary(content string) error {
	return m.writeFile("summary.md", content)
}

// WriteLog writes to logs/ subdirectory.
func (m *WorkdirManager) WriteLog(name string, content []byte) error {
	if m.current == "" {
		return fmt.Errorf("no workdir created")
	}
	path := filepath.Join(m.current, "logs", name)
	return os.WriteFile(path, content, 0644)
}

// ReadRequirements reads requirements.md.
func (m *WorkdirManager) ReadRequirements() (string, error) {
	return m.readFile("requirements.md")
}

// ReadArchitectAnalysis reads architect-analysis.md.
func (m *WorkdirManager) ReadArchitectAnalysis() (string, error) {
	return m.readFile("architect-analysis.md")
}

// ReadStep reads step_N.md.
func (m *WorkdirManager) ReadStep(n int) (string, error) {
	filename := fmt.Sprintf("step_%d.md", n)
	return m.readFile(filename)
}

// ReadStepImpl reads step_N_impl.md.
func (m *WorkdirManager) ReadStepImpl(n int) (string, error) {
	filename := fmt.Sprintf("step_%d_impl.md", n)
	return m.readFile(filename)
}

// ReadStepValidation reads step_N_valid.md.
func (m *WorkdirManager) ReadStepValidation(n int) (string, error) {
	filename := fmt.Sprintf("step_%d_valid.md", n)
	return m.readFile(filename)
}

// ReadFinalValidation reads final_validation.md.
func (m *WorkdirManager) ReadFinalValidation() (string, error) {
	return m.readFile("final_validation.md")
}

// ReadSummary reads summary.md.
func (m *WorkdirManager) ReadSummary() (string, error) {
	return m.readFile("summary.md")
}

// GetLogPath returns the full path for a log file.
func (m *WorkdirManager) GetLogPath(name string) string {
	if m.current == "" {
		return ""
	}
	return filepath.Join(m.current, "logs", name)
}

// SummaryPath returns the path to summary.md.
func (m *WorkdirManager) SummaryPath() string {
	if m.current == "" {
		return ""
	}
	return filepath.Join(m.current, "summary.md")
}

// RequirementsPath returns the path to requirements.md.
func (m *WorkdirManager) RequirementsPath() string {
	if m.current == "" {
		return ""
	}
	return filepath.Join(m.current, "requirements.md")
}

// StepPath returns the path to step_N.md.
func (m *WorkdirManager) StepPath(n int) string {
	if m.current == "" {
		return ""
	}
	return filepath.Join(m.current, fmt.Sprintf("step_%d.md", n))
}

// writeFile writes content to a file in the workdir.
func (m *WorkdirManager) writeFile(filename, content string) error {
	if m.current == "" {
		return fmt.Errorf("no workdir created")
	}
	path := filepath.Join(m.current, filename)
	return os.WriteFile(path, []byte(content), 0644)
}

// readFile reads content from a file in the workdir.
func (m *WorkdirManager) readFile(filename string) (string, error) {
	if m.current == "" {
		return "", fmt.Errorf("no workdir created")
	}
	path := filepath.Join(m.current, filename)
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ListFiles returns all files in the workdir.
func (m *WorkdirManager) ListFiles() ([]string, error) {
	if m.current == "" {
		return nil, fmt.Errorf("no workdir created")
	}

	var files []string
	err := filepath.Walk(m.current, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			rel, _ := filepath.Rel(m.current, path)
			files = append(files, rel)
		}
		return nil
	})
	return files, err
}

// HasSummary returns true if summary.md exists.
func (m *WorkdirManager) HasSummary() bool {
	if m.current == "" {
		return false
	}
	path := filepath.Join(m.current, "summary.md")
	_, err := os.Stat(path)
	return err == nil
}

// GetLatestWorkdir returns the most recent workdir.
func GetLatestWorkdir(baseDir string) (string, error) {
	workdirBase := filepath.Join(baseDir, ".claude", "workdir")

	entries, err := os.ReadDir(workdirBase)
	if err != nil {
		return "", err
	}

	var latest string
	var latestTime time.Time

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if info.ModTime().After(latestTime) {
			latestTime = info.ModTime()
			latest = filepath.Join(workdirBase, entry.Name())
		}
	}

	if latest == "" {
		return "", fmt.Errorf("no workdir found")
	}

	return latest, nil
}
