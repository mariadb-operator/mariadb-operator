package backup

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/go-logr/logr"
)

const timeLayout = time.RFC3339

// time.Now cannot be mocked globablly, this is to allow overriding the now func from tests
var now = time.Now

type backupDiff struct {
	fileName string
	diff     time.Duration
}

// GetBackupTargetFile finds the backup file with the closest date to the target recovery time.
func GetBackupTargetFile(backupFileNames []string, targetRecoveryTime time.Time, logger logr.Logger) (string, error) {
	var backupDiffs []backupDiff
	for _, file := range backupFileNames {
		backupDate, err := parseDateInBackupFile(file)
		if err != nil {
			logger.Error(err, "error parsing backup date. Skipping", "file", file)
			continue
		}
		diff := backupDate.Sub(targetRecoveryTime).Abs()
		if diff == 0 {
			return file, nil
		}
		backupDiffs = append(backupDiffs, backupDiff{
			fileName: file,
			diff:     diff,
		})
	}
	if len(backupDiffs) == 0 {
		return "", errors.New("no valid backup files were found")
	}

	sort.Slice(backupDiffs, func(i, j int) bool {
		return backupDiffs[i].diff < backupDiffs[j].diff
	})
	return backupDiffs[0].fileName, nil
}

// GetOldBackupFiles determines which backup files should be deleted according with the retention policy.
func GetOldBackupFiles(backupFileNames []string, maxRetention time.Duration, logger logr.Logger) []string {
	var oldBackups []string
	now := now()
	for _, file := range backupFileNames {
		backupDate, err := parseDateInBackupFile(file)
		if err != nil {
			logger.Error(err, "error parsing backup date. Skipping", "file", file)
			continue
		}
		if now.Sub(backupDate) > maxRetention {
			oldBackups = append(oldBackups, file)
		}
	}
	return oldBackups
}

// IsValidBackupFile determines whether a backup file name is valid.
func IsValidBackupFile(fileName string) bool {
	if !strings.HasPrefix(fileName, "backup.") || !strings.HasSuffix(fileName, ".sql") {
		return false
	}
	_, err := parseDateInBackupFile(fileName)
	return err == nil
}

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

func parseDateInBackupFile(fileName string) (time.Time, error) {
	parts := strings.Split(fileName, ".")
	if len(parts) != 3 {
		return time.Time{}, fmt.Errorf("invalid backup file name: %s", fileName)
	}
	return ParseBackupDate(parts[1])
}
