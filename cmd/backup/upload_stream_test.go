package backup

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUploadStreamCommandInheritsGeneratedBackupFlags(t *testing.T) {
	for _, flagName := range []string{
		"max-retention",
		"physical-backup-meta",
		"physical-backup-name",
		"physical-backup-namespace",
	} {
		assert.NotNil(t, uploadStreamCommand.Flag(flagName), "expected upload-stream to accept --%s", flagName)
	}
}
