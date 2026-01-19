package backup

import (
	"errors"
	"fmt"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/go-logr/logr"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v25/api/v1alpha1"
	mdbtime "github.com/mariadb-operator/mariadb-operator/v25/pkg/time"
)

type BackupProcessor interface {
	GetBackupTargetFile(backupFileNames []string, targetRecoveryTime time.Time, logger logr.Logger) (string, error)
	GetOldBackupFiles(backupFileNames []string, maxRetention time.Duration, logger logr.Logger) []string
	IsValidBackupFile(fileName string) bool
	ParseCompressionAlgorithm(fileName string) (mariadbv1alpha1.CompressAlgorithm, error)
	GetUncompressedBackupFile(compressedBackupFile string) (string, error)
	parseDateInBackupFile(fileName string) (time.Time, error)
}

type backupDiff struct {
	fileName string
	diff     time.Duration
}

// LogicalBackupProcessor processes logical backups.
type LogicalBackupProcessor struct{}

// NewLogicalBackupProcessor creates a new LogicalBackupProcessor.
func NewLogicalBackupProcessor() BackupProcessor {
	return &LogicalBackupProcessor{}
}

// GetBackupTargetFile finds the backup file with the closest date to the target recovery time.
func (p *LogicalBackupProcessor) GetBackupTargetFile(backupFileNames []string, targetRecoveryTime time.Time,
	logger logr.Logger) (string, error) {
	var backupDiffs []backupDiff
	for _, file := range backupFileNames {
		backupDate, err := p.parseDateInBackupFile(file)
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
func (p *LogicalBackupProcessor) GetOldBackupFiles(backupFileNames []string, maxRetention time.Duration, logger logr.Logger) []string {
	var oldBackups []string
	now := now()
	for _, file := range backupFileNames {
		backupDate, err := p.parseDateInBackupFile(file)
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
func (p *LogicalBackupProcessor) IsValidBackupFile(fileName string) bool {
	if !strings.HasPrefix(fileName, "backup.") || !strings.HasSuffix(fileName, ".sql") {
		return false
	}
	_, err := p.ParseCompressionAlgorithm(fileName)
	if err != nil {
		return false
	}
	_, err = p.parseDateInBackupFile(fileName)
	return err == nil
}

// ParseCompressionAlrogrithm gets the compression algorithm from the backup file name.
// Supports both new format (backup.timestamp.sql.gz) and legacy format (backup.timestamp.gzip.sql).
func (p *LogicalBackupProcessor) ParseCompressionAlgorithm(fileName string) (mariadbv1alpha1.CompressAlgorithm, error) {
	parts := strings.Split(fileName, ".")
	// No compression: backup.timestamp.sql
	if len(parts) == 3 {
		return mariadbv1alpha1.CompressNone, nil
	}
	if len(parts) != 4 {
		return mariadbv1alpha1.CompressAlgorithm(""), fmt.Errorf("invalid backup file name: %s", fileName)
	}

	// New format: backup.timestamp.sql.gz (extension at the end)
	if parts[2] == "sql" {
		return mariadbv1alpha1.CompressionFromExtension(parts[3])
	}

	// Legacy format: backup.timestamp.gzip.sql (algorithm name in the middle)
	calg := mariadbv1alpha1.CompressAlgorithm(parts[2])
	if err := calg.Validate(); err != nil {
		return "", err
	}
	return calg, nil
}

// GetUncompressedBackupFile get the backup file without compression extension.
// Supports both new format (backup.timestamp.sql.gz) and legacy format (backup.timestamp.gzip.sql).
func (p *LogicalBackupProcessor) GetUncompressedBackupFile(compressedBackupFile string) (string, error) {
	parts := strings.Split(compressedBackupFile, ".")
	if len(parts) != 4 {
		return "", fmt.Errorf("invalid compressed backup file name: %s", compressedBackupFile)
	}

	// New format: backup.timestamp.sql.gz -> backup.timestamp.sql
	if parts[2] == "sql" {
		if _, err := mariadbv1alpha1.CompressionFromExtension(parts[3]); err != nil {
			return "", err
		}
		return fmt.Sprintf("%s.%s.%s", parts[0], parts[1], parts[2]), nil
	}

	// Legacy format: backup.timestamp.gzip.sql -> backup.timestamp.sql
	calg := mariadbv1alpha1.CompressAlgorithm(parts[2])
	if err := calg.Validate(); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s.%s.%s", parts[0], parts[1], parts[3]), nil
}

func (p *LogicalBackupProcessor) parseDateInBackupFile(fileName string) (time.Time, error) {
	parts := strings.Split(fileName, ".")
	if len(parts) != 3 && len(parts) != 4 {
		return time.Time{}, fmt.Errorf("invalid backup file name: %s", fileName)
	}
	return ParseBackupDate(parts[1])
}

// PhysicalBackupProcessor processes physical backups.
type PhysicalBackupProcessor struct {
	isValidBackupFileFn     func(fileName string) bool
	parseDateInBackupFileFn func(fileName string) (time.Time, error)
}

// PhysicalBackupProcsssorOpt is an option to modify PhysicalBackupProcessor behavior.
type PhysicalBackupProcsssorOpt func(*PhysicalBackupProcessor)

// WithPhysicalBackupValidationFn configures a custom backup name validation function.
func WithPhysicalBackupValidationFn(validationFn func(fileName string) bool) PhysicalBackupProcsssorOpt {
	return func(pbp *PhysicalBackupProcessor) {
		pbp.isValidBackupFileFn = validationFn
	}
}

// WithPhysicalBackupParseDateFn configures a custom backup parse date validation function.
func WithPhysicalBackupParseDateFn(parseDateFn func(fileName string) (time.Time, error)) PhysicalBackupProcsssorOpt {
	return func(pbp *PhysicalBackupProcessor) {
		pbp.parseDateInBackupFileFn = parseDateFn
	}
}

// NewPhysicalBackupProcessor creates a new PhysicalBackupProcessor.
func NewPhysicalBackupProcessor(opts ...PhysicalBackupProcsssorOpt) BackupProcessor {
	processor := &PhysicalBackupProcessor{}
	for _, setOpt := range opts {
		setOpt(processor)
	}
	return processor
}

// GetBackupTargetFile finds the backup file with the closest date to the target recovery time.
func (p *PhysicalBackupProcessor) GetBackupTargetFile(backupFileNames []string, targetRecoveryTime time.Time,
	logger logr.Logger) (string, error) {
	var backupDiffs []backupDiff
	for _, file := range backupFileNames {
		backupDate, err := p.parseDateInBackupFile(file)
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
func (p *PhysicalBackupProcessor) GetOldBackupFiles(backupFileNames []string, maxRetention time.Duration, logger logr.Logger) []string {
	var oldBackups []string
	now := now()
	for _, file := range backupFileNames {
		backupDate, err := p.parseDateInBackupFile(file)
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
func (p *PhysicalBackupProcessor) IsValidBackupFile(fileName string) bool {
	if validationFn := p.isValidBackupFileFn; validationFn != nil {
		return validationFn(fileName)
	}
	baseName := path.Base(fileName)

	if !strings.HasPrefix(baseName, "physicalbackup-") {
		return false
	}

	_, err := p.ParseCompressionAlgorithm(baseName)
	if err != nil {
		return false
	}

	_, err = p.parseDateInBackupFile(baseName)
	return err == nil
}

// ParseCompressionAlrogrithm gets the compression algorithm from the backup file name.
func (p *PhysicalBackupProcessor) ParseCompressionAlgorithm(fileName string) (mariadbv1alpha1.CompressAlgorithm, error) {
	parts := strings.Split(fileName, ".")
	if len(parts) == 2 {
		return mariadbv1alpha1.CompressNone, nil
	}
	if len(parts) != 3 {
		return mariadbv1alpha1.CompressAlgorithm(""), fmt.Errorf("invalid backup file name: %s", fileName)
	}

	calg, err := mariadbv1alpha1.CompressionFromExtension(parts[2])
	if err != nil {
		return "", err
	}
	if err := calg.Validate(); err != nil {
		return "", err
	}
	return calg, nil
}

// GetUncompressedBackupFile get the backup file without compression extension.
func (p *PhysicalBackupProcessor) GetUncompressedBackupFile(compressedBackupFile string) (string, error) {
	parts := strings.Split(compressedBackupFile, ".")
	if len(parts) != 3 {
		return "", fmt.Errorf("invalid compressed physical backup file: %s", compressedBackupFile)
	}
	alg, err := mariadbv1alpha1.CompressionFromExtension(parts[2])
	if err != nil {
		return "", err
	}
	if err := alg.Validate(); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s.%s", parts[0], parts[1]), nil
}

func (p *PhysicalBackupProcessor) parseDateInBackupFile(fileName string) (time.Time, error) {
	if parseDateFn := p.parseDateInBackupFileFn; parseDateFn != nil {
		return parseDateFn(fileName)
	}
	baseName := path.Base(fileName)

	parts := strings.Split(baseName, ".")
	if len(parts) == 0 {
		return time.Time{}, fmt.Errorf("invalid physical backup prefix: %s", fileName)
	}
	base := parts[0]
	if !strings.HasPrefix(base, "physicalbackup-") {
		return time.Time{}, fmt.Errorf("invalid physical backup prefix: %s", fileName)
	}
	date := strings.TrimPrefix(base, "physicalbackup-")
	return mdbtime.Parse(date)
}
