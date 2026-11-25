package command

import (
	"testing"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestSqlExecCommand(t *testing.T) {
	mariadb := &mariadbv1alpha1.MariaDB{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "default",
		},
		Spec: mariadbv1alpha1.MariaDBSpec{
			Port: 3306,
		},
	}
	cmd, err := NewSqlCommand(
		WithSqlUserEnv("USER"),
		WithSqlPasswordEnv("PASS"),
		WithSqlFile("/opt/job.sql"),
	)
	assert.NoError(t, err)

	got, err := cmd.ExecCommand(mariadb)
	assert.NoError(t, err)

	want := "set -euo pipefail;" +
		"echo '⚙️ Executing SQL script';" +
		"mariadb --user=${USER} --password=${PASS} --host=test.default.svc.cluster.local --port=3306 " +
		"--abort-source-on-error -e 'source /opt/job.sql'"
	assert.Equal(t, []string{"bash", "-c"}, got.Command)
	assert.Equal(t, []string{want}, got.Args)
}
