package backup

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

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

// GetFilePath returns the path to a backup file.
func GetFilePath(path, fileName string) string {
	if filepath.IsAbs(fileName) {
		return fileName
	}
	return filepath.Join(path, fileName)
}

// ErrPathTraversal is returned when a filename contains path traversal sequences.
var ErrPathTraversal = errors.New("path traversal detected in filename")

// ValidateFilename checks that a filename does not contain path traversal sequences.
func ValidateFilename(fileName string) error {
	if strings.Contains(fileName, "..") {
		return fmt.Errorf("%w: filename contains '..' sequence: %s", ErrPathTraversal, fileName)
	}
	if filepath.IsAbs(fileName) {
		return fmt.Errorf("%w: filename is an absolute path: %s", ErrPathTraversal, fileName)
	}
	cleaned := filepath.Clean(fileName)
	if strings.HasPrefix(cleaned, "..") {
		return fmt.Errorf("%w: cleaned path escapes base directory: %s", ErrPathTraversal, cleaned)
	}
	return nil
}

// GetFilePathSafe returns a validated path to a backup file, ensuring it does not escape the base directory.
func GetFilePathSafe(basePath, fileName string) (string, error) {
	if err := ValidateFilename(fileName); err != nil {
		return "", err
	}
	absBase, err := filepath.Abs(basePath)
	if err != nil {
		return "", fmt.Errorf("error resolving base path: %v", err)
	}
	fullPath := filepath.Join(absBase, fileName)
	absPath, err := filepath.Abs(fullPath)
	if err != nil {
		return "", fmt.Errorf("error resolving full path: %v", err)
	}
	if !strings.HasPrefix(absPath, absBase+string(filepath.Separator)) && absPath != absBase {
		return "", fmt.Errorf("%w: resolved path escapes base directory", ErrPathTraversal)
	}
	return absPath, nil
}
