package pitr

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/go-logr/logr"
)

const timeLayout = time.RFC3339

type backupDiff struct {
	fileName string
	diff     time.Duration
}

// GetTargetRecoveryFile finds the backup file with the closest date to the target recovery time.
func GetTargetRecoveryFile(backupFileNames []string, targetRecoveryTime time.Time, logger logr.Logger) (string, error) {
	var backupDiffs []backupDiff
	for _, file := range backupFileNames {
		backupDate, err := parseBackupDate(file)
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

// IsValidBackupFile determines whether a backup file name is valid.
func IsValidBackupFile(fileName string) bool {
	if !strings.HasPrefix(fileName, "backup.") || !strings.HasSuffix(fileName, ".sql") {
		return false
	}
	_, err := parseBackupDate(fileName)
	return err == nil
}

// FormatBackupDate formats a time with the layout compatible with this module.
func FormatBackupDate(t time.Time) string {
	return t.Format(timeLayout)
}

func parseBackupDate(backupFileName string) (time.Time, error) {
	parts := strings.Split(backupFileName, ".")
	if len(parts) != 3 {
		return time.Time{}, fmt.Errorf("invalid file name: %s", backupFileName)
	}
	t, err := time.Parse(timeLayout, parts[1])
	if err != nil {
		return time.Time{}, fmt.Errorf("error parsing file date: %v", err)
	}
	return t, nil
}
