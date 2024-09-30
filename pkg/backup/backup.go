package backup

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/go-logr/logr"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
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
	_, err := ParseCompressionAlgorithm(fileName)
	if err != nil {
		return false
	}
	_, err = parseDateInBackupFile(fileName)
	return err == nil
}

// ParseCompressionAlrogrithm gets the compression algorithm from the backup file name.
func ParseCompressionAlgorithm(fileName string) (mariadbv1alpha1.CompressAlgorithm, error) {
	parts := strings.Split(fileName, ".")
	if len(parts) == 3 {
		return mariadbv1alpha1.CompressNone, nil
	}
	if len(parts) != 4 {
		return mariadbv1alpha1.CompressAlgorithm(""), fmt.Errorf("invalid backup file name: %s", fileName)
	}

	calg := mariadbv1alpha1.CompressAlgorithm(parts[2])
	if err := calg.Validate(); err != nil {
		return "", err
	}
	return calg, nil
}

// GetUncompressedBackupFile returns the file without the compression extension.
// It will return an error if the file does not have compression. You may check this with ParseCompressionAlgorithm.
func GetUncompressedBackupFile(compressedBackupFile string) (string, error) {
	parts := strings.Split(compressedBackupFile, ".")
	if len(parts) != 4 {
		return "", fmt.Errorf("invalid compressed backup file name: %s", compressedBackupFile)
	}

	calg := mariadbv1alpha1.CompressAlgorithm(parts[2])
	if err := calg.Validate(); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s.%s.%s", parts[0], parts[1], parts[3]), nil
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
	if len(parts) != 3 && len(parts) != 4 {
		return time.Time{}, fmt.Errorf("invalid backup file name: %s", fileName)
	}
	return ParseBackupDate(parts[1])
}
