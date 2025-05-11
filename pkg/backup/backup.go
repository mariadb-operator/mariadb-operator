package backup

import (
	"fmt"
	"path/filepath"
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
