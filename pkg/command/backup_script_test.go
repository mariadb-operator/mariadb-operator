package command

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// The tests in this file run the generated bash scripts against stub binaries, so they cover shell semantics
// (PIPESTATUS, set -u, pipe ordering) that string assertions cannot.

func writeStub(t *testing.T, path, body string) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, []byte("#!/usr/bin/env bash\n"+body+"\n"), 0o755))
}

type scriptRun struct {
	output   string
	exitCode int
}

func runScript(t *testing.T, cmd *Command, binDir string, env ...string) scriptRun {
	t.Helper()
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash not available")
	}
	require.Len(t, cmd.Args, 1)
	c := exec.CommandContext(t.Context(), bash, "-c", cmd.Args[0])
	c.Env = append([]string{"PATH=" + binDir + ":" + os.Getenv("PATH")}, env...)
	out, err := c.CombinedOutput()
	run := scriptRun{output: string(out)}
	if exitErr, ok := err.(*exec.ExitError); ok {
		run.exitCode = exitErr.ExitCode()
	} else {
		require.NoError(t, err, string(out))
	}
	return run
}

type restoreScriptEnv struct {
	binDir      string
	operatorBin string
	dataDir     string
	stagingDir  string
	streamedLog string
}

// newRestoreScriptEnv provides stubs for the operator stream command, mbstream and mariadb-backup. The stream stub
// records each invocation and exits with $STREAM_EXIT_CODE; mbstream extracts an ibdata1 file and exits with
// $MBSTREAM_EXIT_CODE.
func newRestoreScriptEnv(t *testing.T) restoreScriptEnv {
	t.Helper()
	dir := t.TempDir()
	env := restoreScriptEnv{
		binDir:      filepath.Join(dir, "bin"),
		operatorBin: filepath.Join(dir, "operator", "mariadb-operator"),
		dataDir:     filepath.Join(dir, "data"),
		stagingDir:  filepath.Join(dir, "backup", "full"),
		streamedLog: filepath.Join(dir, "streamed.log"),
	}
	require.NoError(t, os.MkdirAll(env.binDir, 0o755))
	require.NoError(t, os.MkdirAll(filepath.Dir(env.operatorBin), 0o755))

	writeStub(t, env.operatorBin, `echo streamed >> `+env.streamedLog+`
echo xbstream-data
exit "${STREAM_EXIT_CODE:-0}"`)
	writeStub(t, filepath.Join(env.binDir, "mbstream"), `while [ $# -gt 0 ]; do
	if [ "$1" = "-C" ]; then target="$2"; fi
	shift
done
cat > /dev/null
touch "$target/ibdata1"
exit "${MBSTREAM_EXIT_CODE:-0}"`)
	writeStub(t, filepath.Join(env.binDir, "mariadb-backup"), "exit 0")
	return env
}

func (e restoreScriptEnv) restoreCommand(t *testing.T, opts ...MariaDBBackupRestoreOpt) *Command {
	t.Helper()
	backupCmd := &BackupCommand{
		BackupOpts: BackupOpts{
			Path:              filepath.Dir(e.stagingDir),
			TargetFilePath:    filepath.Join(filepath.Dir(e.stagingDir), "0-backup-target.txt"),
			BackupFullDirPath: e.stagingDir,
			BackupContentType: mariadbv1alpha1.BackupContentTypePhysical,
		},
	}
	cmd, err := backupCmd.MariadbBackupRestore(&mariadbv1alpha1.MariaDB{}, e.dataDir,
		append([]MariaDBBackupRestoreOpt{WithOperatorBinaryPath(e.operatorBin)}, opts...)...)
	require.NoError(t, err)
	return cmd
}

func (e restoreScriptEnv) streamCount(t *testing.T) int {
	t.Helper()
	data, err := os.ReadFile(e.streamedLog)
	if os.IsNotExist(err) {
		return 0
	}
	require.NoError(t, err)
	return strings.Count(string(data), "streamed")
}

func TestStreamingRestoreScriptExitCodes(t *testing.T) {
	tests := []struct {
		name     string
		direct   bool
		env      []string
		wantExit int
	}{
		{name: "direct success", direct: true, wantExit: 0},
		{name: "direct stream failure", direct: true, env: []string{"STREAM_EXIT_CODE=3"}, wantExit: 3},
		{name: "direct mbstream failure", direct: true, env: []string{"MBSTREAM_EXIT_CODE=4"}, wantExit: 4},
		{name: "staging success", direct: false, wantExit: 0},
		{name: "staging stream failure", direct: false, env: []string{"STREAM_EXIT_CODE=3"}, wantExit: 3},
		{name: "staging mbstream failure", direct: false, env: []string{"MBSTREAM_EXIT_CODE=4"}, wantExit: 4},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newRestoreScriptEnv(t)
			var opts []MariaDBBackupRestoreOpt
			if tt.direct {
				opts = append(opts, WithDataDir(e.dataDir))
			}
			run := runScript(t, e.restoreCommand(t, opts...), e.binDir, tt.env...)
			assert.Equal(t, tt.wantExit, run.exitCode, run.output)

			if tt.direct {
				_, err := os.Stat(filepath.Join(e.dataDir, ".mariadb-operator-restore-complete"))
				assert.Equal(t, tt.wantExit == 0, err == nil, "restore marker presence")
			}
		})
	}
}

func TestStreamingRestoreScriptRestoreMarker(t *testing.T) {
	tests := []struct {
		name           string
		cleanupDataDir bool
		wantStreams    int
	}{
		{name: "bootstrap retry reuses a completed restore", cleanupDataDir: false, wantStreams: 0},
		{name: "replica recovery ignores a marker from an earlier restore", cleanupDataDir: true, wantStreams: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newRestoreScriptEnv(t)
			require.NoError(t, os.MkdirAll(e.dataDir, 0o755))
			for _, f := range []string{".mariadb-operator-restore-complete", "ibdata1", "stale.ibd"} {
				require.NoError(t, os.WriteFile(filepath.Join(e.dataDir, f), nil, 0o644))
			}

			cmd := e.restoreCommand(t, WithDataDir(e.dataDir), WithCleanupDataDir(tt.cleanupDataDir))
			run := runScript(t, cmd, e.binDir)
			assert.Equal(t, 0, run.exitCode, run.output)
			assert.Equal(t, tt.wantStreams, e.streamCount(t), run.output)

			_, err := os.Stat(filepath.Join(e.dataDir, "stale.ibd"))
			assert.Equal(t, tt.cleanupDataDir, os.IsNotExist(err), "stale data file removed")
		})
	}
}

func TestStreamingBackupScriptRecordsProducerExitBeforeEOF(t *testing.T) {
	tests := []struct {
		name         string
		producerExit string
		wantExit     int
	}{
		{name: "producer success", producerExit: "0", wantExit: 0},
		{name: "producer failure", producerExit: "7", wantExit: 7},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			binDir := filepath.Join(dir, "bin")
			require.NoError(t, os.MkdirAll(binDir, 0o755))
			operatorBin := filepath.Join(dir, "mariadb-operator")
			seenExit := filepath.Join(dir, "seen-exit")

			writeStub(t, filepath.Join(binDir, "mariadb-backup"), `echo partial-backup
exit `+tt.producerExit)
			// The uploader stub reads stdin to EOF and then records what it finds in the producer exit file, which
			// is what upload-stream does before completing the multipart upload.
			writeStub(t, operatorBin, `while [ $# -gt 0 ]; do
	if [ "$1" = "--producer-exit-file" ]; then exit_file="$2"; fi
	shift
done
cat > /dev/null
cat "$exit_file" > `+seenExit)

			backupCmd := &BackupCommand{
				BackupOpts: BackupOpts{
					CommandOpts: CommandOpts{
						UserEnv:     "MARIADB_OPERATOR_USER",
						PasswordEnv: "MARIADB_OPERATOR_PASSWORD",
					},
					Path:                 dir,
					TargetFilePath:       filepath.Join(dir, "0-backup-target.txt"),
					BackupContentType:    mariadbv1alpha1.BackupContentTypePhysical,
					MaxRetentionDuration: time.Hour,
					S3:                   true,
					S3Bucket:             "bucket",
					S3Endpoint:           "endpoint",
				},
			}
			mariadb := &mariadbv1alpha1.MariaDB{ObjectMeta: metav1.ObjectMeta{Name: "mariadb", Namespace: "default"}}
			cmd, err := backupCmd.MariadbStreamingBackup(mariadb, "physicalbackup-20260101000000.xb.gz", 0, operatorBin)
			require.NoError(t, err)

			// A stale exit code from an earlier run must not be picked up.
			require.NoError(t, os.WriteFile(filepath.Join(dir, "0-backup-producer-exit.txt"), []byte("0"), 0o644))

			run := runScript(t, cmd, binDir, "MARIADB_OPERATOR_USER=root", "MARIADB_OPERATOR_PASSWORD=secret")
			assert.Equal(t, tt.wantExit, run.exitCode, run.output)
			seen, err := os.ReadFile(seenExit)
			require.NoError(t, err, run.output)
			assert.Equal(t, tt.producerExit, string(seen))
		})
	}
}
