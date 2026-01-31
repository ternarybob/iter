package index

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ContextLineage tracks commit history with LLM-generated summaries.
type ContextLineage struct {
	mu          sync.RWMutex
	repoRoot    string
	storagePath string // path to lineage storage directory
	summaries   map[string]*LineageSummary
	llm         *LLMClient
}

// LineageSummary contains a summary of a commit's changes.
type LineageSummary struct {
	CommitHash   string    `json:"commit_hash"`
	ShortHash    string    `json:"short_hash"`
	Author       string    `json:"author"`
	Date         time.Time `json:"date"`
	Message      string    `json:"message"`
	FilesChanged []string  `json:"files_changed"`
	Summary      string    `json:"summary"` // LLM-generated summary
	SummarizedAt time.Time `json:"summarized_at"`
	SummaryModel string    `json:"summary_model"` // Model used for summary
}

// CommitInfo contains raw commit information from git.
type CommitInfo struct {
	Hash         string
	ShortHash    string
	Author       string
	Date         time.Time
	Message      string
	FilesChanged []string
	Diff         string // Truncated diff
}

// NewContextLineage creates a new context lineage tracker.
func NewContextLineage(repoRoot, storagePath string, llm *LLMClient) *ContextLineage {
	return &ContextLineage{
		repoRoot:    repoRoot,
		storagePath: storagePath,
		summaries:   make(map[string]*LineageSummary),
		llm:         llm,
	}
}

// Load loads existing summaries from disk.
func (l *ContextLineage) Load() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Create directory if it doesn't exist
	if err := os.MkdirAll(l.storagePath, 0755); err != nil {
		return fmt.Errorf("create lineage directory: %w", err)
	}

	// Load all summary files
	entries, err := os.ReadDir(l.storagePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read lineage directory: %w", err)
	}

	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		path := filepath.Join(l.storagePath, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		var summary LineageSummary
		if err := json.Unmarshal(data, &summary); err != nil {
			continue
		}

		l.summaries[summary.CommitHash] = &summary
	}

	return nil
}

// GetSummary returns a summary for a commit hash.
func (l *ContextLineage) GetSummary(hash string) (*LineageSummary, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	summary, ok := l.summaries[hash]
	return summary, ok
}

// HasSummary checks if a summary exists for a commit.
func (l *ContextLineage) HasSummary(hash string) bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	_, ok := l.summaries[hash]
	return ok
}

// StoreSummary saves a summary to disk.
func (l *ContextLineage) StoreSummary(summary *LineageSummary) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.summaries[summary.CommitHash] = summary

	// Save to file
	path := filepath.Join(l.storagePath, summary.ShortHash+".json")
	data, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal summary: %w", err)
	}

	if err := os.MkdirAll(l.storagePath, 0755); err != nil {
		return fmt.Errorf("create lineage directory: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write summary: %w", err)
	}

	return nil
}

// ParseCommit parses commit information from git.
func (l *ContextLineage) ParseCommit(hash string) (*CommitInfo, error) {
	// Get commit details
	cmd := exec.Command("git", "-C", l.repoRoot, "show",
		"--no-patch",
		"--format=%H%n%h%n%an%n%aI%n%s",
		hash)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git show: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) < 5 {
		return nil, fmt.Errorf("unexpected git output format")
	}

	date, _ := time.Parse(time.RFC3339, lines[3])

	info := &CommitInfo{
		Hash:      lines[0],
		ShortHash: lines[1],
		Author:    lines[2],
		Date:      date,
		Message:   strings.Join(lines[4:], "\n"),
	}

	// Get changed files
	cmd = exec.Command("git", "-C", l.repoRoot, "diff-tree",
		"--no-commit-id", "--name-only", "-r", hash)
	output, err = cmd.Output()
	if err == nil {
		info.FilesChanged = strings.Split(strings.TrimSpace(string(output)), "\n")
	}

	// Get truncated diff (first 5000 chars)
	cmd = exec.Command("git", "-C", l.repoRoot, "show",
		"--stat", "--patch", "-M", hash)
	output, err = cmd.Output()
	if err == nil {
		info.Diff = string(output)
		if len(info.Diff) > 5000 {
			info.Diff = info.Diff[:5000] + "\n... (truncated)"
		}
	}

	return info, nil
}

// SummarizeCommit generates an LLM summary for a commit.
func (l *ContextLineage) SummarizeCommit(hash string) (*LineageSummary, error) {
	// Check if already summarized
	if summary, ok := l.GetSummary(hash); ok {
		return summary, nil
	}

	// Parse commit info
	info, err := l.ParseCommit(hash)
	if err != nil {
		return nil, err
	}

	summary := &LineageSummary{
		CommitHash:   info.Hash,
		ShortHash:    info.ShortHash,
		Author:       info.Author,
		Date:         info.Date,
		Message:      info.Message,
		FilesChanged: info.FilesChanged,
		SummarizedAt: time.Now(),
	}

	// Generate LLM summary if client is available
	if l.llm != nil {
		prompt := fmt.Sprintf(`Summarize this git commit in 1-2 sentences. Focus on WHAT changed and WHY.

Commit: %s
Author: %s
Message: %s

Files changed:
%s

Diff:
%s

Summary:`,
			info.ShortHash,
			info.Author,
			info.Message,
			strings.Join(info.FilesChanged, "\n"),
			info.Diff)

		llmSummary, model, err := l.llm.Generate(prompt)
		if err == nil {
			summary.Summary = strings.TrimSpace(llmSummary)
			summary.SummaryModel = model
		} else {
			// Fallback to commit message
			summary.Summary = info.Message
			summary.SummaryModel = "fallback"
		}
	} else {
		// No LLM - use commit message as summary
		summary.Summary = info.Message
		summary.SummaryModel = "none"
	}

	// Store summary
	if err := l.StoreSummary(summary); err != nil {
		return summary, err
	}

	return summary, nil
}

// ScanNewCommits scans for new commits and generates summaries.
// maxCommits limits how many commits to scan (0 = all).
func (l *ContextLineage) ScanNewCommits(maxCommits int) ([]*LineageSummary, error) {
	// Get recent commit hashes
	limit := "100"
	if maxCommits > 0 && maxCommits < 100 {
		limit = fmt.Sprintf("%d", maxCommits)
	}

	cmd := exec.Command("git", "-C", l.repoRoot, "log",
		"--format=%H", "-n", limit)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git log: %w", err)
	}

	hashes := strings.Split(strings.TrimSpace(string(output)), "\n")
	var newSummaries []*LineageSummary

	for _, hash := range hashes {
		if hash == "" {
			continue
		}

		// Skip if already summarized
		if l.HasSummary(hash) {
			continue
		}

		summary, err := l.SummarizeCommit(hash)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to summarize %s: %v\n", hash[:7], err)
			continue
		}

		newSummaries = append(newSummaries, summary)

		if maxCommits > 0 && len(newSummaries) >= maxCommits {
			break
		}
	}

	return newSummaries, nil
}

// GetRecentHistory returns recent commit summaries.
func (l *ContextLineage) GetRecentHistory(limit int) ([]*LineageSummary, error) {
	// Get recent commit hashes in order
	cmd := exec.Command("git", "-C", l.repoRoot, "log",
		"--format=%H", "-n", fmt.Sprintf("%d", limit))
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git log: %w", err)
	}

	hashes := strings.Split(strings.TrimSpace(string(output)), "\n")
	var summaries []*LineageSummary

	for _, hash := range hashes {
		if hash == "" {
			continue
		}

		// Get or create summary
		summary, ok := l.GetSummary(hash)
		if !ok {
			// Parse basic info without LLM summary
			info, err := l.ParseCommit(hash)
			if err != nil {
				continue
			}
			summary = &LineageSummary{
				CommitHash:   info.Hash,
				ShortHash:    info.ShortHash,
				Author:       info.Author,
				Date:         info.Date,
				Message:      info.Message,
				FilesChanged: info.FilesChanged,
				Summary:      info.Message,
				SummaryModel: "pending",
			}
		}

		summaries = append(summaries, summary)
	}

	return summaries, nil
}

// FormatHistory formats commit history as markdown.
func FormatHistory(summaries []*LineageSummary) string {
	if len(summaries) == 0 {
		return "No commit history available.\n"
	}

	var sb strings.Builder
	sb.WriteString("# Commit History\n\n")

	for _, s := range summaries {
		sb.WriteString(fmt.Sprintf("## %s - %s\n", s.ShortHash, s.Date.Format("2006-01-02 15:04")))
		sb.WriteString(fmt.Sprintf("**Author**: %s\n", s.Author))
		sb.WriteString(fmt.Sprintf("**Message**: %s\n", s.Message))
		if s.Summary != s.Message && s.Summary != "" {
			sb.WriteString(fmt.Sprintf("**Summary**: %s\n", s.Summary))
		}
		if len(s.FilesChanged) > 0 {
			sb.WriteString("**Files**:\n")
			for _, f := range s.FilesChanged {
				sb.WriteString(fmt.Sprintf("- %s\n", f))
			}
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// Stats returns statistics about the lineage.
func (l *ContextLineage) Stats() LineageStats {
	l.mu.RLock()
	defer l.mu.RUnlock()

	stats := LineageStats{
		TotalSummaries: len(l.summaries),
	}

	for _, s := range l.summaries {
		if s.SummaryModel != "none" && s.SummaryModel != "fallback" && s.SummaryModel != "pending" {
			stats.LLMSummaries++
		}
	}

	return stats
}

// LineageStats contains statistics about the lineage tracker.
type LineageStats struct {
	TotalSummaries int `json:"total_summaries"`
	LLMSummaries   int `json:"llm_summaries"`
}
