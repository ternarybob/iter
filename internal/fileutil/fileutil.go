// Package fileutil provides file system utilities.
package fileutil

import (
	"os"
	"path/filepath"
)

// Exists checks if a path exists.
func Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// IsDir checks if a path is a directory.
func IsDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// IsFile checks if a path is a regular file.
func IsFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// EnsureDir creates a directory if it doesn't exist.
func EnsureDir(path string) error {
	return os.MkdirAll(path, 0755)
}

// ReadFile reads a file and returns its content.
func ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

// WriteFile writes content to a file.
func WriteFile(path string, content []byte) error {
	dir := filepath.Dir(path)
	if err := EnsureDir(dir); err != nil {
		return err
	}
	return os.WriteFile(path, content, 0644)
}

// CopyFile copies a file.
func CopyFile(src, dst string) error {
	content, err := ReadFile(src)
	if err != nil {
		return err
	}
	return WriteFile(dst, content)
}

// RemoveAll removes a path and all its children.
func RemoveAll(path string) error {
	return os.RemoveAll(path)
}

// Abs returns the absolute path.
func Abs(path string) (string, error) {
	return filepath.Abs(path)
}

// Rel returns the relative path.
func Rel(basepath, targpath string) (string, error) {
	return filepath.Rel(basepath, targpath)
}

// Join joins path elements.
func Join(elem ...string) string {
	return filepath.Join(elem...)
}

// Base returns the last element of path.
func Base(path string) string {
	return filepath.Base(path)
}

// Dir returns all but the last element of path.
func Dir(path string) string {
	return filepath.Dir(path)
}

// Ext returns the file extension.
func Ext(path string) string {
	return filepath.Ext(path)
}
