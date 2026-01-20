package index

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Walker traverses directories to find files for indexing.
type Walker struct {
	opts IndexOptions
}

// NewWalker creates a new directory walker.
func NewWalker(opts IndexOptions) *Walker {
	return &Walker{opts: opts}
}

// WalkFunc is called for each file.
type WalkFunc func(path string, content []byte) error

// Walk traverses a directory and calls fn for each matching file.
func (w *Walker) Walk(ctx context.Context, root string, fn WalkFunc) error {
	root, err := filepath.Abs(root)
	if err != nil {
		return err
	}

	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		// Check context
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Skip directories that match exclude patterns
		if d.IsDir() {
			relPath, _ := filepath.Rel(root, path)
			for _, pattern := range w.opts.ExcludePatterns {
				if matchGlob(relPath+"/", pattern) || matchGlob(d.Name()+"/", pattern) {
					return filepath.SkipDir
				}
			}
			return nil
		}

		// Check if file should be included
		relPath, _ := filepath.Rel(root, path)
		if !w.shouldInclude(relPath) {
			return nil
		}

		// Check file size
		info, err := d.Info()
		if err != nil {
			return nil
		}
		if w.opts.MaxFileSize > 0 && info.Size() > w.opts.MaxFileSize {
			return nil
		}

		// Read file
		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		// Skip binary files
		if isBinary(content) {
			return nil
		}

		return fn(relPath, content)
	})
}

// shouldInclude checks if a file should be indexed.
func (w *Walker) shouldInclude(path string) bool {
	// Check exclude patterns
	for _, pattern := range w.opts.ExcludePatterns {
		if matchGlob(path, pattern) {
			return false
		}
	}

	// Check include patterns
	if len(w.opts.IncludePatterns) == 0 {
		return true
	}

	for _, pattern := range w.opts.IncludePatterns {
		if matchGlob(path, pattern) {
			return true
		}
	}

	return false
}

// matchGlob performs simple glob matching.
func matchGlob(path, pattern string) bool {
	// Normalize paths
	path = strings.ReplaceAll(path, "\\", "/")
	pattern = strings.ReplaceAll(pattern, "\\", "/")

	// Handle ** pattern
	if strings.Contains(pattern, "**") {
		return matchDoubleGlob(path, pattern)
	}

	return matchSimpleGlob(path, pattern)
}

// matchSimpleGlob matches without **.
func matchSimpleGlob(path, pattern string) bool {
	pi := 0 // pattern index
	si := 0 // string index

	for pi < len(pattern) && si < len(path) {
		switch pattern[pi] {
		case '*':
			// Match any sequence of characters (except /)
			pi++
			if pi >= len(pattern) {
				// Trailing * - match rest if no /
				return !strings.Contains(path[si:], "/")
			}
			// Find next literal in pattern
			for si < len(path) && path[si] != '/' {
				if matchSimpleGlob(path[si:], pattern[pi:]) {
					return true
				}
				si++
			}
			return matchSimpleGlob(path[si:], pattern[pi:])
		case '?':
			// Match any single character (except /)
			if path[si] == '/' {
				return false
			}
			pi++
			si++
		default:
			if pattern[pi] != path[si] {
				return false
			}
			pi++
			si++
		}
	}

	// Skip trailing wildcards in pattern
	for pi < len(pattern) && pattern[pi] == '*' {
		pi++
	}

	return pi >= len(pattern) && si >= len(path)
}

// matchDoubleGlob matches with ** pattern.
func matchDoubleGlob(path, pattern string) bool {
	// Split pattern on **
	parts := strings.Split(pattern, "**")

	// Handle leading part
	if parts[0] != "" {
		if !strings.HasPrefix(path, strings.TrimSuffix(parts[0], "/")) &&
			!matchSimpleGlob(path, parts[0]+"*") {
			return false
		}
	}

	// Handle trailing part
	if len(parts) > 1 && parts[len(parts)-1] != "" {
		trailing := parts[len(parts)-1]
		trailing = strings.TrimPrefix(trailing, "/")
		if !matchSimpleGlob(filepath.Base(path), trailing) &&
			!strings.HasSuffix(path, trailing) {
			return false
		}
	}

	// ** matches everything in between
	return true
}

// isBinary checks if content appears to be binary.
func isBinary(content []byte) bool {
	// Check for null bytes
	maxCheck := 8000
	if len(content) < maxCheck {
		maxCheck = len(content)
	}

	for i := 0; i < maxCheck; i++ {
		if content[i] == 0 {
			return true
		}
	}

	return false
}

// ListFiles returns all files matching the options without reading content.
func (w *Walker) ListFiles(ctx context.Context, root string) ([]string, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}

	var files []string

	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if d.IsDir() {
			relPath, _ := filepath.Rel(root, path)
			for _, pattern := range w.opts.ExcludePatterns {
				if matchGlob(relPath+"/", pattern) || matchGlob(d.Name()+"/", pattern) {
					return filepath.SkipDir
				}
			}
			return nil
		}

		relPath, _ := filepath.Rel(root, path)
		if w.shouldInclude(relPath) {
			files = append(files, relPath)
		}

		return nil
	})

	return files, err
}

// FileInfo contains basic file information.
type FileInfo struct {
	Path     string
	Size     int64
	ModTime  int64
	Language string
}

// ListFilesWithInfo returns files with metadata.
func (w *Walker) ListFilesWithInfo(ctx context.Context, root string) ([]FileInfo, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}

	var files []FileInfo

	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if d.IsDir() {
			relPath, _ := filepath.Rel(root, path)
			for _, pattern := range w.opts.ExcludePatterns {
				if matchGlob(relPath+"/", pattern) || matchGlob(d.Name()+"/", pattern) {
					return filepath.SkipDir
				}
			}
			return nil
		}

		relPath, _ := filepath.Rel(root, path)
		if !w.shouldInclude(relPath) {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return nil
		}

		if w.opts.MaxFileSize > 0 && info.Size() > w.opts.MaxFileSize {
			return nil
		}

		files = append(files, FileInfo{
			Path:     relPath,
			Size:     info.Size(),
			ModTime:  info.ModTime().Unix(),
			Language: LanguageFromPath(relPath),
		})

		return nil
	})

	return files, err
}
