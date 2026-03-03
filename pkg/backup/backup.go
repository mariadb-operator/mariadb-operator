package backup

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// ErrPathTraversal is returned when a filename contains path traversal sequences.
var ErrPathTraversal = errors.New("path traversal detected in filename")

const timeLayout = time.RFC3339

// time.Now cannot be mocked globally, this is to allow overriding the now func from tests
var now = time.Now

// FormatBackupDate formats a time with the layout compatible with this module.
func FormatBackupDate(t time.Time) string {
	return t.Format(timeLayout)
}

// ParseBackupDate parses a time string with the layout compatible with this module.
func ParseBackupDate(timeRaw string) (time.Time, error) {
	t, err := time.Parse(timeLayout, timeRaw)
	if err != nil {
		return time.Time{}, fmt.Errorf("error parsing backup date: %v", err)
	}
	return t, nil
}

// ValidateFilename checks if a filename is safe and doesn't contain path traversal sequences.
// Returns an error if the filename could be used for path traversal attacks.
func ValidateFilename(fileName string) error {
	// Check for obvious path traversal patterns
	if strings.Contains(fileName, "..") {
		return fmt.Errorf("%w: filename contains '..'", ErrPathTraversal)
	}

	// Clean the filename and check if it tries to escape
	cleaned := filepath.Clean(fileName)
	if strings.HasPrefix(cleaned, "..") {
		return fmt.Errorf("%w: cleaned path escapes base directory", ErrPathTraversal)
	}

	// Check for absolute paths when we expect relative filenames
	if filepath.IsAbs(fileName) {
		return fmt.Errorf("%w: absolute paths not allowed", ErrPathTraversal)
	}

	return nil
}

// GetFilePath returns the path to a backup file.
func GetFilePath(path, fileName string) string {
	if filepath.IsAbs(fileName) {
		return fileName
	}
	return filepath.Join(path, fileName)
}

// GetFilePathSafe returns the path to a backup file after validating for path traversal.
// Returns an error if the filename contains path traversal sequences.
func GetFilePathSafe(basePath, fileName string) (string, error) {
	if err := ValidateFilename(fileName); err != nil {
		return "", err
	}

	fullPath := filepath.Join(basePath, fileName)

	// Additional check: ensure the resolved path is within the base path
	absBase, err := filepath.Abs(basePath)
	if err != nil {
		return "", fmt.Errorf("error resolving base path: %w", err)
	}
	absPath, err := filepath.Abs(fullPath)
	if err != nil {
		return "", fmt.Errorf("error resolving file path: %w", err)
	}

	// Ensure the path is within the base directory
	if !strings.HasPrefix(absPath, absBase+string(filepath.Separator)) && absPath != absBase {
		return "", fmt.Errorf("%w: path escapes base directory", ErrPathTraversal)
	}

	return fullPath, nil
}
